package orchestrator

// Model tiering runs intermediate tool-using iterations on a Spirit's cheaper DraftModel while the
// primary Model always produces the final answer. It is driven entirely by the Spirit configuration
// (SpiritModel.DraftModel): no global policy. When DraftModel is unset or equals the primary, the
// loop uses a single model — behaviour identical to before.

func (r *ReasoningService) tierDraftModel(req *ReasoningRequest) string {
	if req.Spirit.ModelConfig.DraftModel != "" {
		return req.Spirit.ModelConfig.DraftModel
	}
	return req.Spirit.ModelConfig.Model
}

// tieringActive reports whether the Spirit configured a distinct draft model, i.e. intermediate
// iterations should run on the cheaper model and the final answer be re-synthesized on the primary.
func (r *ReasoningService) tieringActive(req *ReasoningRequest) bool {
	draft := req.Spirit.ModelConfig.DraftModel
	return draft != "" && draft != req.Spirit.ModelConfig.Model
}
