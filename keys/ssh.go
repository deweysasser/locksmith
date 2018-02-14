package keys

func parseSshPublicKey(content string) Key {
	return GenericKeyImpl{content}
}
