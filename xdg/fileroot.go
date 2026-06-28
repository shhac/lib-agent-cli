package xdg

import output "github.com/shhac/lib-agent-output"

// Root names a host directory for exposure through an MCP file tool (see
// lib-agent-mcp's WithFileRoots). name is the stable label the agent sees and
// addresses files under; dir is the actual host directory, which the agent
// never sees. Constructing roots here keeps the family's file-access surface
// defined in one place — apps pick which of their directories to expose and
// under what name, then hand the result to the MCP server.
func Root(name, dir string) output.FileRoot {
	return output.FileRoot{Name: name, Path: dir}
}
