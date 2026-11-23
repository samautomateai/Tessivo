package server

import (
	"context"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/service"
)

func TestChatApprovalsWaitAnswerAndTimeOut(t *testing.T) {
	var a chatApprovals
	go func() {
		for !a.answer("c/1", service.ChatToolAnswer{Allow: true}) {
			time.Sleep(time.Millisecond)
		}
	}()
	if got := a.wait(context.Background(), "c/1", time.Minute); !got.Allow {
		t.Fatal("answer not delivered")
	}
	if got := a.wait(context.Background(), "c/2", 10*time.Millisecond); got.Allow {
		t.Fatal("a timed-out call must be denied")
	}
	if a.answer("c/3", service.ChatToolAnswer{Allow: true}) {
		t.Fatal("answered a call nobody waits on")
	}
}

func TestAnswerChatToolRoute(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	if code, _ := call(t, ts, "POST", "/v1/chats/aaaaaaaaaaaaaaaa/approvals/x", `{"allow":true}`); code != 404 {
		t.Fatalf("nothing waiting: %d", code)
	}
}
