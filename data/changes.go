package data

type Change struct {
	Type        string
	Account     ID
	Add, Remove []KeyBindingImpl

	// ManualAdd holds bindings the operator asked for directly, via `add`.
	//
	// It is separate from Add because the two have different lifetimes.  Add
	// and Remove are *derived*: `plan` recomputes them from policy on every run
	// and drops whatever is no longer warranted.  ManualAdd is an instruction,
	// so `plan` carries it forward untouched.  Keeping both in one slice meant
	// plan could not tell them apart, and had to choose between discarding the
	// operator's request and accumulating derived work that no longer applied.
	ManualAdd []KeyBindingImpl `json:",omitempty"`

	// Manual marks a change the operator asked for directly.  Changes are keyed
	// by account, so both kinds land in the same file; without this, re-planning
	// silently discarded whatever `add` had just written -- and `plan` is
	// exactly the command someone runs next to inspect what they asked for.
	Manual bool `json:",omitempty"`
}

// Additions is every binding this change would add: what the operator asked for
// plus what policy implies.  Callers that act on a change want both; only
// `plan`, which owns one half and not the other, looks at the fields directly.
func (c Change) Additions() []KeyBindingImpl {
	if len(c.ManualAdd) == 0 {
		return c.Add
	}

	seen := make(map[KeyBindingImpl]bool, len(c.Add)+len(c.ManualAdd))
	out := make([]KeyBindingImpl, 0, len(c.Add)+len(c.ManualAdd))
	for _, b := range append(append([]KeyBindingImpl{}, c.ManualAdd...), c.Add...) {
		if !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	return out
}

// Id is deliberately on the value receiver: changes are stored and listed by
// value, and only a value receiver puts Id in the method set of both Change and
// *Change.  Without it the library cannot see a Change as a data.Ider and falls
// back to hashing the JSON, which keys changes by content instead of by
// account.
func (c Change) Id() ID {
	return c.Account
}
