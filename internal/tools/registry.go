package tools

import "slices"

var allTools []Tool

func register(t Tool) {
	allTools = append(allTools, t)
}

func All() []Tool {
	return slices.Clone(allTools)
}

func ByID(id ToolID) (Tool, bool) {
	for _, tool := range allTools {
		if tool.ID == id {
			return tool, true
		}
	}
	return Tool{}, false
}
