package agent

import (
	"errors"
	"fmt"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// CheckpointVersion is the format of Checkpoint written by this code.
const CheckpointVersion = 1

// States of a pending call.
const (
	StateQueued  = "queued"
	StateRunning = "running" // marked before running; still set after a crash means "outcome unknown"
)

// Checkpoint is everything needed to continue a run: the transcript, the
// counters and the calls of the current step that have not finished.
type Checkpoint struct {
	Version    int                       `json:"version"`
	RunID      string                    `json:"runId"`
	NotebookID string                    `json:"notebookId,omitempty"`
	ScheduleID string                    `json:"scheduleId,omitempty"`
	Status     string                    `json:"status"`
	Roots      []string                  `json:"roots,omitempty"`
	Messages   []llm.Message             `json:"messages"`
	Steps      int                       `json:"steps"`
	Tokens     int                       `json:"tokens"`
	ElapsedMS  int64                     `json:"elapsedMs"`
	Attempts   []notebook.AttemptRecord  `json:"attempts,omitempty"`
	ToolCalls  []notebook.ToolCallRecord `json:"toolCalls,omitempty"`
	Approvals  []notebook.ApprovalRecord `json:"approvals,omitempty"`
	Pending    []PendingCall             `json:"pending,omitempty"`
	RunRules   map[string]string         `json:"runRules,omitempty"` // "remember for this run" answers
	CreatedAt  string                    `json:"createdAt,omitempty"`
	UpdatedAt  string                    `json:"updatedAt,omitempty"`
}

// PendingCall is a tool call of the current step that has not finished.
type PendingCall struct {
	Call       llm.ToolCall `json:"call"`
	Decision   Decision     `json:"decision"`
	ApprovalID string       `json:"approvalId,omitempty"`
	Answer     string       `json:"answer,omitempty"` // allow or deny, set when the user answers
	State      string       `json:"state"`
}

// ErrPaused marks a run that stopped to wait for the user's approval.
var ErrPaused = errors.New("run paused for approval")

// PausedError lists the calls waiting for an answer.
type PausedError struct {
	Approvals []PendingCall
}

func (e *PausedError) Error() string {
	return fmt.Sprintf("%s: %d tool call(s) need an answer", ErrPaused, len(e.Approvals))
}

// Is reports a match for ErrPaused.
func (e *PausedError) Is(target error) bool { return target == ErrPaused }
