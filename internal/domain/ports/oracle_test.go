package ports

import "testing"

func TestOracleUsage_Add(t *testing.T) {
	u := &OracleUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	u.Add(OracleUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5})
	if u.PromptTokens != 13 {
		t.Errorf("expected PromptTokens=13, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 7 {
		t.Errorf("expected CompletionTokens=7, got %d", u.CompletionTokens)
	}
	if u.TotalTokens != 20 {
		t.Errorf("expected TotalTokens=20, got %d", u.TotalTokens)
	}
}
