package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/samautomateai/tessivo/internal/service"
)

// outputData prints an output's structured result as indented JSON, the
// form `tess output edit` reads back.
func outputData(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	info, err := svc.GetOutput(a.nb, a.out)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(info.Data, &v); err != nil {
		return fmt.Errorf("output %s: %w", a.out, err)
	}
	enc := json.NewEncoder(env.Stdout)
	enc.SetIndent("", "  ")
	return wrapWrite(enc.Encode(v))
}

// outputEdit saves the JSON in a file (or stdin for "-") as a new version of
// the output, checked against its work type; no model is called.
func outputEdit(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	var r io.Reader
	switch src := a.rest[0]; {
	case src == "-" && (env.Stdin == nil || env.StdinIsTTY):
		return usagef("output edit -: pipe the JSON in")
	case src == "-":
		r = env.Stdin
	default:
		f, err := os.Open(src) //nolint:gosec // G304: the file the user named
		if err != nil {
			return fmt.Errorf("output edit: %w", err)
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	data, err := io.ReadAll(io.LimitReader(r, service.MaxEditedOutput+1))
	if err != nil {
		return fmt.Errorf("output edit: read: %w", err)
	}
	info, err := svc.EditOutput(a.nb, a.out, data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "saved v%d as %s\n", info.Version, info.ID)
	return wrapWrite(err)
}
