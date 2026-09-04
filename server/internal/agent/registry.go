// agent/registry.go：工具扁平注册表。
//
// 单一注册点，按名查找/子集。工具在 tools.Build 装配后 Register；
// SubAgent 通过 Subset 取工具子集。
package agent

// ToolRegistry 工具注册表（名→Tool）。
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry 创建注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register 注册工具（按 Info().Name 索引，后注册覆盖同名）。
func (r *ToolRegistry) Register(t Tool) {
	info := t.Info()
	if info == nil {
		return
	}
	r.tools[info.Name] = t
}

// Get 按名查找工具。不存在返回 nil。
func (r *ToolRegistry) Get(name string) Tool {
	return r.tools[name]
}

// Subset 按名取工具子集（供 SubAgent 限定可用工具）。
func (r *ToolRegistry) Subset(names []string) []Tool {
	out := make([]Tool, 0, len(names))
	for _, n := range names {
		if t, ok := r.tools[n]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Names 返回所有已注册工具名。
func (r *ToolRegistry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for n := range r.tools {
		out = append(out, n)
	}
	return out
}
