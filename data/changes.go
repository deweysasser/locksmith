package data

type Change struct {
	Type        string
	Account     ID
	Add, Remove []KeyBindingImpl
}

func (c *Change) Id() ID {
	return c.Account
}
