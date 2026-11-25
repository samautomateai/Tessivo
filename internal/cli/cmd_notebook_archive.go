package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samautomateai/tessivo/internal/atomicfile"
	"github.com/samautomateai/tessivo/internal/service"
)

// folderService opens a notebook folder through the service: the folder's
// parent acts as the workspace and its name as the notebook ID.
func folderService(dir string, env Env) (*service.Service, string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", fmt.Errorf("notebook %s: %w", dir, err)
	}
	f, err := loadConfig(env)
	if err != nil {
		return nil, "", err
	}
	svc, err := newService(f, env)
	if err != nil {
		return nil, "", err
	}
	svc.Workspace = filepath.Dir(abs)
	return svc, filepath.Base(abs), nil
}

// notebookExport writes a notebook folder as an archive:
// tess notebook export <notebook> [--outputs] [-o file].
func notebookExport(args []string, env Env) error {
	fs := newFlags("notebook export", env)
	outputs := fs.Bool("outputs", false, "include the saved outputs")
	out := fs.String("o", "", "archive file (default <notebook>.tessivo.zip here)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("notebook export: want <notebook> [--outputs] [-o file]")
	}
	svc, id, err := folderService(pos[0], env)
	if err != nil {
		return err
	}
	path := *out
	if path == "" {
		path = id + ".tessivo.zip"
	}
	var buf bytes.Buffer
	if err := svc.ExportNotebook(id, *outputs, &buf); err != nil {
		return err
	}
	if err := atomicfile.Write(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	_, err = fmt.Fprintf(env.Stdout, "wrote %s\n", path)
	return wrapWrite(err)
}

// notebookImport adds an archive to the configured workspace:
// tess notebook import <file>.
func notebookImport(args []string, env Env) error {
	if len(args) != 1 {
		return usagef("notebook import: want <archive file>")
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	file, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	defer func() { _ = file.Close() }()
	st, err := file.Stat()
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	info, err := svc.ImportNotebook(file, st.Size())
	if err != nil {
		return err
	}
	relink := 0
	for _, c := range info.Context {
		if c.NeedsRelink {
			relink++
		}
	}
	msg := fmt.Sprintf("imported %q as %s", info.Name, filepath.Join(svc.Workspace, info.ID))
	switch relink {
	case 0:
	case 1:
		msg += "; 1 source needs relinking (tess context relink)"
	default:
		msg += fmt.Sprintf("; %d sources need relinking (tess context relink)", relink)
	}
	_, err = fmt.Fprintln(env.Stdout, msg+"; its schedules are off")
	return wrapWrite(err)
}

// contextRelink points a source that needs relinking at a path:
// tess context relink <notebook> <source id> <path>.
func contextRelink(args []string, env Env) error {
	if len(args) != 3 {
		return usagef("context relink: want <notebook> <source id> <absolute path>")
	}
	svc, id, err := folderService(args[0], env)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(args[2])
	if err != nil {
		return fmt.Errorf("relink: %w", err)
	}
	if _, err := svc.RelinkContext(id, args[1], path); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "relinked %s to %s\n", args[1], path)
	return wrapWrite(err)
}
