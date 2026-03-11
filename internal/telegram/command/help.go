package command

import (
	"context"
	"fmt"
	"strings"
)

type HelpCmd struct {
	registry *Registry
}

func (c *HelpCmd) Name() string        { return "help" }
func (c *HelpCmd) Description() string  { return "Show available commands" }

func (c *HelpCmd) Execute(_ context.Context, _ string) (string, error) {
	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, cmd := range c.registry.All() {
		fmt.Fprintf(&b, "  /%s - %s\n", cmd.Name(), cmd.Description())
	}
	return b.String(), nil
}
