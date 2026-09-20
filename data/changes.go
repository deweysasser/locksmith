package data

type Change struct {
	Type        string
	Account     ID
	Add, Remove []KeyBindingImpl
}

// Id is deliberately on the value receiver: changes are stored and listed by
// value, and only a value receiver puts Id in the method set of both Change and
// *Change.  Without it the library cannot see a Change as a data.Ider and falls
// back to hashing the JSON, which keys changes by content instead of by
// account.
func (c Change) Id() ID {
	return c.Account
}
