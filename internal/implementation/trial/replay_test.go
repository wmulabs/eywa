package trial

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type stubChronicleReader struct {
	byMemoryKey []*entities.Chronicle
	byTimeRange []*entities.Chronicle
	err         error
	gotMemory   string
	gotStart    time.Time
	gotEnd      time.Time
}

func (s *stubChronicleReader) FindByMemoryKey(_ context.Context, memoryKey string) ([]*entities.Chronicle, error) {
	s.gotMemory = memoryKey
	return s.byMemoryKey, s.err
}

func (s *stubChronicleReader) FindByTimeRange(_ context.Context, start, end time.Time) ([]*entities.Chronicle, error) {
	s.gotStart, s.gotEnd = start, end
	return s.byTimeRange, s.err
}

func chronicle(id, eventKey, userInput, finalResponse string) *entities.Chronicle {
	c := &entities.Chronicle{ID: id, EventKey: eventKey}
	c.Event.UserInput = userInput
	c.Processing.FinalResponse = finalResponse
	return c
}

func TestCasesFromChronicles(t *testing.T) {
	in := []*entities.Chronicle{
		chronicle("c1", "user_message", "hello", "hi there"),
		nil, // skipped
		chronicle("c2", "user_message", "", "orphan"), // skipped: no user input
		chronicle("c3", "support", "help me", "sure"),
	}

	cases := CasesFromChronicles(in)
	if len(cases) != 2 {
		t.Fatalf("expected 2 replayable cases, got %d", len(cases))
	}
	if cases[0].ID != "c1" || cases[0].EventType != "user_message" || cases[0].UserMessage != "hello" || cases[0].Baseline != "hi there" {
		t.Errorf("unexpected first case: %+v", cases[0])
	}
	if cases[1].ID != "c3" || cases[1].Baseline != "sure" {
		t.Errorf("unexpected second case: %+v", cases[1])
	}
}

func TestLoadReplayCases_ByMemoryKey(t *testing.T) {
	reader := &stubChronicleReader{byMemoryKey: []*entities.Chronicle{chronicle("c1", "user_message", "hi", "yo")}}
	cases, err := LoadReplayCases(context.Background(), reader, ReplayOptions{MemoryKey: "user:1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.gotMemory != "user:1" {
		t.Errorf("expected lookup by memory key 'user:1', got %q", reader.gotMemory)
	}
	if len(cases) != 1 || cases[0].UserMessage != "hi" {
		t.Errorf("unexpected cases: %+v", cases)
	}
}

func TestLoadReplayCases_ByTimeRange(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	reader := &stubChronicleReader{byTimeRange: []*entities.Chronicle{chronicle("c1", "e", "q", "a")}}

	cases, err := LoadReplayCases(context.Background(), reader, ReplayOptions{Start: start, End: end})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reader.gotStart.Equal(start) || !reader.gotEnd.Equal(end) {
		t.Error("expected time-range lookup with the provided bounds")
	}
	if len(cases) != 1 {
		t.Errorf("expected 1 case, got %d", len(cases))
	}
}

func TestLoadReplayCases_NoSelector(t *testing.T) {
	if _, err := LoadReplayCases(context.Background(), &stubChronicleReader{}, ReplayOptions{}); err == nil {
		t.Error("expected an error when neither memory key nor time range is given")
	}
}

func TestLoadReplayCases_ReaderError(t *testing.T) {
	reader := &stubChronicleReader{err: errors.New("db down")}
	if _, err := LoadReplayCases(context.Background(), reader, ReplayOptions{MemoryKey: "user:1"}); err == nil {
		t.Error("expected the reader error to propagate")
	}
}
