package services

import (
	"context"
	"errors"
	"testing"
)

func TestCompileNodeLimit(t *testing.T) {
	ctx := WithCompileNodeLimit(context.Background(), 10)
	if err := checkCompileNodeLimit(ctx, 10); err != nil {
		t.Fatalf("at the limit: %v", err)
	}
	if err := checkCompileNodeLimit(ctx, 11); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("over the limit: %v", err)
	}
	if err := checkCompileNodeLimit(context.Background(), 9999); err != nil {
		t.Fatalf("no limit set: %v", err)
	}
}

func TestReportValidationNamesOutcome(t *testing.T) {
	var got []CompileEvent
	ctx := WithCompileProgress(context.Background(), func(ev CompileEvent) { got = append(got, ev) })
	reportValidation(ctx, 0, []string{"bad ref"}, 3)
	reportValidation(ctx, 1, nil, 3)
	if len(got) != 2 || got[0].Status != "failed" || got[0].Items[0] != "bad ref" || got[1].Status != "done" || got[1].Count != 3 {
		t.Fatalf("events: %+v", got)
	}
}
