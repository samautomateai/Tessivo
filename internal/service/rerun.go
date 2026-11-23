package service

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/config"
)

// RunInfo is what a past run used: enough to run it again.
type RunInfo struct {
	RunID      string            `json:"runId"`
	Prompt     string            `json:"prompt"`
	PromptName string            `json:"promptName,omitempty"`
	Vars       map[string]string `json:"vars,omitempty"`
	Model      string            `json:"model"`
	Mode       string            `json:"mode,omitempty"`
}

// OutputRun returns the run that produced an output.
func (s *Service) OutputRun(notebookRef, outputID string) (RunInfo, error) {
	nb, err := s.openNotebookRef(notebookRef)
	if err != nil {
		return RunInfo{}, err
	}
	o, _, err := nb.GetOutput(outputID)
	if err != nil {
		return RunInfo{}, fmt.Errorf("output %s: %w", outputID, ErrNotFound)
	}
	run, err := nb.LoadRun(o.RunID)
	if err != nil {
		return RunInfo{}, fmt.Errorf("the run behind output %s: %w", outputID, ErrNotFound)
	}
	return RunInfo{
		RunID: run.ID, Prompt: run.Prompt, PromptName: run.PromptName, Vars: run.Vars,
		Model: run.Settings.Model, Mode: run.Mode,
	}, nil
}

// RerunRequest builds a request that runs an output's run again, on model
// when given (else with the settings that apply now).
func (s *Service) RerunRequest(notebookRef, outputID, model string) (RunRequest, error) {
	ri, err := s.OutputRun(notebookRef, outputID)
	if err != nil {
		return RunRequest{}, err
	}
	nb, err := s.openNotebookRef(notebookRef)
	if err != nil {
		return RunRequest{}, err
	}
	req := RunRequest{
		Notebook: nb, Prompt: ri.Prompt, PromptName: ri.PromptName, Vars: ri.Vars, Save: true,
		Overrides: config.RunSettings{Model: model},
	}
	if ri.PromptName != "" {
		if _, err := nb.LoadPrompt(ri.PromptName); err != nil {
			req.PromptName = "" // the saved prompt is gone: run the text it had
		}
	}
	return req, nil
}
