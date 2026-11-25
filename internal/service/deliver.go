package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/samautomateai/tessivo/internal/atomicfile"
	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/deliver"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// Delivery records one thing sent: an issue made from a finding, or a page
// (Finding -1). Kept in outputs/<id>.delivered next to the output.
type Delivery struct {
	Integration string `json:"integration"`
	Kind        string `json:"kind"`
	Finding     int    `json:"finding"`
	ID          string `json:"id"`
	URL         string `json:"url"`
	At          string `json:"at"`
}

// PushResult is what a push sent and which findings it skipped as sent before.
type PushResult struct {
	Sent    []Delivery `json:"sent"`
	Skipped []int      `json:"skipped"`
}

// Bounds on deliveries.
const (
	maxDeliveries  = 1000 // records kept per output, newest
	maxParentHops  = 50   // versions walked back to find a page to replace
	deliveryPage   = -1
	deliveriesFile = ".delivered"
	// MaxPushIssues bounds the issues one push creates.
	MaxPushIssues  = 100
	auditKeyOutput = "output"
	auditKeyKind   = "kind"
)

// Deliveries lists what was sent from an output, oldest first.
func (s *Service) Deliveries(notebookID, outputID string) ([]Delivery, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return nil, err
	}
	return readDeliveries(ref.nb, outputID)
}

func deliveriesPath(nb *notebook.Notebook, outputID string) string {
	return filepath.Join(nb.Dir, notebook.OutputsDir, outputID+deliveriesFile)
}

func readDeliveries(nb *notebook.Notebook, outputID string) ([]Delivery, error) {
	b, err := os.ReadFile(deliveriesPath(nb, outputID))
	if errors.Is(err, fs.ErrNotExist) {
		return []Delivery{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deliveries: %w", err)
	}
	var ds []Delivery
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, fmt.Errorf("deliveries: %w", err)
	}
	return ds, nil
}

// addDeliveries appends records; s.deliverMu is held.
func addDeliveries(nb *notebook.Notebook, outputID string, add []Delivery) error {
	ds, err := readDeliveries(nb, outputID)
	if err != nil {
		return err
	}
	ds = append(ds, add...)
	if len(ds) > maxDeliveries {
		ds = ds[len(ds)-maxDeliveries:]
	}
	b, err := json.Marshal(ds)
	if err != nil {
		return fmt.Errorf("deliveries: %w", err)
	}
	if err := atomicfile.Write(deliveriesPath(nb, outputID), b, 0o600); err != nil {
		return fmt.Errorf("deliveries: %w", err)
	}
	return nil
}

// client opens an integration with its token; a missing token says how to set it.
func (s *Service) client(name string, issues bool) (config.Integration, *deliver.Client, error) {
	in, err := s.integration(name)
	if err != nil {
		return in, nil, err
	}
	if in.Kind != config.IntegrationFolder && issues != in.ForIssues() {
		what := "pages"
		if issues {
			what = "issues"
		}
		return in, nil, invalidf("integration %s (%s) takes no %s", name, in.Kind, what)
	}
	if issues && in.Kind == config.IntegrationFolder {
		return in, nil, invalidf("integration %s is a folder; publish to it instead", name)
	}
	token := ""
	if in.TokenRef != "" {
		if s.Secrets != nil {
			token, err = s.Secrets.Get(in.TokenRef)
		}
		if s.Secrets == nil || err != nil || token == "" {
			return in, nil, invalidf("integration %s has no token; store it with: tess secret set %s", name,
				in.TokenRef)
		}
	}
	c, err := deliver.New(in, token)
	if err != nil {
		return in, nil, fmt.Errorf("integration %s: %w", name, err)
	}
	return in, c, nil
}

// PushIssues files the picked findings (all when indexes is empty) as issues.
// Findings already sent to this integration are skipped unless again is set.
// On an error, what was sent before it is recorded and returned.
func (s *Service) PushIssues(ctx context.Context, notebookID, outputID, integration string, indexes []int,
	again bool,
) (PushResult, error) {
	issues, err := s.Issues(notebookID, outputID)
	if err != nil {
		return PushResult{}, err
	}
	in, c, err := s.client(integration, true)
	if err != nil {
		return PushResult{}, err
	}
	picked, err := pickIssues(issues, indexes)
	if err != nil {
		return PushResult{}, err
	}
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return PushResult{}, err
	}
	s.deliverMu.Lock()
	defer s.deliverMu.Unlock()
	before, err := readDeliveries(ref.nb, outputID)
	if err != nil {
		return PushResult{}, err
	}
	res := PushResult{Sent: []Delivery{}, Skipped: []int{}}
	var pushErr error
	for _, is := range picked {
		if !again && slices.ContainsFunc(before, func(d Delivery) bool {
			return d.Integration == integration && d.Finding == is.Index
		}) {
			res.Skipped = append(res.Skipped, is.Index)
			continue
		}
		r, err := c.CreateIssue(ctx, deliver.Issue{Title: is.Title, Body: is.Body, Labels: is.Labels})
		if err != nil {
			pushErr = deliveryErr(err)
			break
		}
		res.Sent = append(res.Sent, Delivery{
			Integration: integration, Kind: in.Kind, Finding: is.Index, ID: r.ID,
			URL: r.URL, At: nowRFC3339(),
		})
	}
	if err := s.recordSent(ref, res.Sent); err != nil {
		return res, err
	}
	return res, pushErr
}

func pickIssues(issues []Issue, indexes []int) ([]Issue, error) {
	if len(indexes) == 0 {
		indexes = make([]int, len(issues))
		for i := range indexes {
			indexes[i] = i
		}
	}
	if len(indexes) > MaxPushIssues {
		return nil, invalidf("at most %d issues at once; pick the findings to send", MaxPushIssues)
	}
	var out []Issue
	for _, i := range indexes {
		if i < 0 || i >= len(issues) {
			return nil, invalidf("finding %d does not exist (this output has %d)", i+1, len(issues))
		}
		out = append(out, issues[i])
	}
	return out, nil
}

// Publish sends the output as a page or file. A page this output, or a
// version it revises, published to the same integration is replaced.
func (s *Service) Publish(ctx context.Context, notebookID, outputID, integration string) (Delivery, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return Delivery{}, err
	}
	in, c, err := s.client(integration, false)
	if err != nil {
		return Delivery{}, err
	}
	s.deliverMu.Lock()
	defer s.deliverMu.Unlock()
	page := deliver.Page{Title: ref.out.Title, Markdown: ref.md, PageID: s.publishedPage(ref, integration)}
	r, err := c.Publish(ctx, page)
	if err != nil {
		return Delivery{}, deliveryErr(err)
	}
	d := Delivery{
		Integration: integration, Kind: in.Kind, Finding: deliveryPage, ID: r.ID, URL: r.URL, At: nowRFC3339(),
	}
	return d, s.recordSent(ref, []Delivery{d})
}

// publishedPage finds the page this output or its earlier versions published to integration.
func (s *Service) publishedPage(ref outputRef, integration string) string {
	id := ref.out.ID
	for range maxParentHops {
		ds, _ := readDeliveries(ref.nb, id)
		for i := len(ds) - 1; i >= 0; i-- {
			if ds[i].Integration == integration && ds[i].Finding == deliveryPage {
				return ds[i].ID
			}
		}
		o, _, err := ref.nb.GetOutput(id)
		if err != nil || o.ParentID == "" {
			return ""
		}
		id = o.ParentID
	}
	return ""
}

// recordSent saves the records and audits them (integration, kind, links; never a token).
func (s *Service) recordSent(ref outputRef, sent []Delivery) error {
	if len(sent) == 0 {
		return nil
	}
	if err := addDeliveries(ref.nb, ref.out.ID, sent); err != nil {
		return err
	}
	urls := make([]string, 0, len(sent))
	for _, d := range sent {
		urls = append(urls, d.URL)
	}
	l, err := audit.Open(s.AuditPath(ref.nb), s.actor())
	if err != nil {
		return fmt.Errorf("deliver: %w", err)
	}
	err = l.Append(audit.KindOutputSent, ref.out.RunID, map[string]any{
		auditKeyOutput: ref.out.ID, "integration": sent[0].Integration, auditKeyKind: sent[0].Kind, "count": len(sent),
		"urls": urls,
	})
	if cerr := l.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("deliver: %w", err)
	}
	return nil
}

// TestIntegration checks an integration's connection and token.
func (s *Service) TestIntegration(ctx context.Context, name string) error {
	in, err := s.integration(name)
	if err != nil {
		return err
	}
	_, c, err := s.client(name, in.ForIssues())
	if err != nil {
		return err
	}
	return deliveryErr(c.Test(ctx))
}

// ErrDelivery marks a failure of the tracker, wiki or folder an output was sent to.
var ErrDelivery = errors.New("delivery failed")

func deliveryErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrDelivery, err)
}
