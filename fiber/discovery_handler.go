package fiber

import (
	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type discoveryHandler struct {
	weave *eywa.Weave
}

func newDiscoveryHandler(weave *eywa.Weave) *discoveryHandler {
	return &discoveryHandler{weave: weave}
}

// get returns all registered engine components needed to populate cockpit configuration forms.
// Covers: actions, scouts/enrichers, classifiers/pathfinders, channels/voices,
// logic routers, and inbound converters/receptors.
func (h *discoveryHandler) get(c *fiberlib.Ctx) error {
	resp := fiberlib.Map{
		"actions":     h.listActions(),
		"scouts":      h.listScouts(),
		"pathfinders": h.listPathfinders(),
		"channels":    h.listChannels(),
		"routers":     h.listRouters(),
		"receptors":   h.weave.GetReceptorNames(),
	}
	return c.JSON(resp)
}

type actionInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	IsCritical  bool           `json:"is_critical"`
	Category    string         `json:"category"`
}

func (h *discoveryHandler) listActions() []actionInfo {
	reg := h.weave.GetActionRegistry()
	if reg == nil {
		return []actionInfo{}
	}
	actions := reg.List()
	out := make([]actionInfo, 0, len(actions))
	for _, a := range actions {
		out = append(out, actionInfo{
			Name:        a.GetName(),
			Description: a.GetDescription(),
			Parameters:  a.GetParameters(),
			IsCritical:  a.IsCritical(),
			Category:    string(a.GetCategory()),
		})
	}
	return out
}

func (h *discoveryHandler) listScouts() []string {
	reg := h.weave.GetScoutRegistry()
	if reg == nil {
		return []string{}
	}
	return reg.ListScouts()
}

func (h *discoveryHandler) listPathfinders() []string {
	reg := h.weave.GetPathfinderRegistry()
	if reg == nil {
		return []string{}
	}
	return reg.List()
}

type channelInfo struct {
	Name        string `json:"name"`
	AutoRespond bool   `json:"auto_respond"`
}

func (h *discoveryHandler) listChannels() []channelInfo {
	reg := h.weave.GetVoiceRegistry()
	if reg == nil {
		return []channelInfo{}
	}
	voices := reg.List()
	out := make([]channelInfo, 0, len(voices))
	for _, v := range voices {
		out = append(out, channelInfo{
			Name:        v.GetName(),
			AutoRespond: v.ShouldAutoRespond(),
		})
	}
	return out
}

func (h *discoveryHandler) listRouters() []string {
	reg := h.weave.GetLogicRouterRegistry()
	if reg == nil {
		return []string{}
	}
	return reg.List()
}
