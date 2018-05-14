package command

import (
	"errors"
	"github.com/deweysasser/locksmith/data"
	"github.com/deweysasser/locksmith/lib"
	"github.com/deweysasser/locksmith/output"
	"github.com/urfave/cli"
)

/* Adds an ID manually to a key.  Note that to get an ID for Amazon generate key pairs we need access to the *private* key.
 * The fingerprint can be extracted with 'openssl.exe pkcs8 -in $KEYFILE -nocrypt -topk8 -outform DER | openssl sha1 -c'
 */
func CmdAddId(c *cli.Context) error {
	outputLevel(c)
	ml := lib.MainLibrary{Path: datadir(c)}

	if len(c.Args()) < 2 {
		output.Error("Requires 2 arguments")
		return errors.New("Requires 2 arguments")
	}

	idToAdd := c.Args()[0]

	filter := buildFilter(c.Args()[1:])

	keys := ml.Keys()
	defer keys.Flush()

	if key, err := findKey(keys, filter); err == nil {
		output.Debug("Adding ID", idToAdd, "to key", key)
		key.Ids.Add(data.ID(idToAdd))
		keys.Store(key)
	} else {
		output.Error("Failed to find 1 key:", err)
		return err
	}

	return nil
}

func findKey(library lib.KeyLibrary, filter Filter) (*data.SSHKey, error) {
	var keys []data.Key

	for k := range library.List() {
		if filter(k) {
			output.Debug("Found matching key ", k)
			keys = append(keys, k.(data.Key))
		}
	}

	switch {
	case len(keys) > 1:
		return nil, errors.New("Only a single key result permitted")
	case len(keys) == 1:
		k0 := keys[0]
		if sshKey, ok := k0.(*data.SSHKey); ok {
			return sshKey, nil
		} else {
			return nil, errors.New("Can only add extra IDs to SSHKey")
		}
	case len(keys) == 0:
		return nil, errors.New("No keys found")
	}
	return nil, errors.New("Internal error")
}
