package claudeconfig

import "testing"

func TestHookCommand(t *testing.T) {
	cases := []struct {
		name string
		hook Hook
		want string
	}{
		{
			name: "python with simple args stays bare",
			hook: Hook{File: "agent-notify.py", Runtime: "python3", Args: []string{"claude", "stop"}},
			want: `python3 "$HOME/.claude/hooks/agent-notify.py" claude stop`,
		},
		{
			name: "node no args",
			hook: Hook{File: "session-init.cjs", Runtime: "node"},
			want: `node "$HOME/.claude/hooks/session-init.cjs"`,
		},
		{
			name: "uv runtime expands to uv run",
			hook: Hook{File: "session-init.py", Runtime: "uv"},
			want: `uv run "$HOME/.claude/hooks/session-init.py"`,
		},
		{
			name: "uv runtime with args",
			hook: Hook{File: "agent-notify.py", Runtime: "uv", Args: []string{"claude", "stop"}},
			want: `uv run "$HOME/.claude/hooks/agent-notify.py" claude stop`,
		},
		{
			name: "empty runtime omits prefix",
			hook: Hook{File: "x.sh"},
			want: `"$HOME/.claude/hooks/x.sh"`,
		},
		{
			name: "arg with shell metacharacters is single-quoted",
			hook: Hook{File: "x.py", Runtime: "python3", Args: []string{"a b", "$(rm -rf /)"}},
			want: `python3 "$HOME/.claude/hooks/x.py" 'a b' '$(rm -rf /)'`,
		},
		{
			name: "embedded single quote is escaped",
			hook: Hook{File: "x.py", Runtime: "python3", Args: []string{"it's"}},
			want: `python3 "$HOME/.claude/hooks/x.py" 'it'\''s'`,
		},
	}
	for _, c := range cases {
		if got := HookCommand(c.hook); got != c.want {
			t.Errorf("%s: HookCommand = %q, want %q", c.name, got, c.want)
		}
	}
}
