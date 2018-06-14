package data

type Change struct {
	Type        string
	Account     ID
	Add, Remove []KeyBindingImpl  // `json:",omitempty"`
}

func (c *Change) Id() ID {
	return c.Account
}


// Merge together 2 changes for the same account
func (c *Change) Merge(other *Change) {
	additions := make(map[ID]KeyBindingImpl)
	removals := make(map[ID]KeyBindingImpl)

	for _, a := range(c.Add) {
		additions[a.KeyID] = a
	}

	for _, r := range(c.Remove) {
		if _, contains := additions[r.KeyID]; !contains {
			removals[r.KeyID] = r
		}
	}


	for _, a := range(other.Add) {
		additions[a.KeyID] = a
	}

	for _, r := range(other.Remove) {
		if _, contains := additions[r.KeyID]; !contains {
			removals[r.KeyID] = r
		}
	}

	var adds, removes []KeyBindingImpl

	for _, a := range additions {
		adds = append(adds, a)
	}

	for _, r := range removes {
		adds = append(adds, r)
	}

	c.Add = adds
	c.Remove = removes
}