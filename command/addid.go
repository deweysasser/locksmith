package command

import (
	"errors"
	"fmt"
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

// findOneKey resolves a filter to exactly one key, of any kind.
//
// Refusing an ambiguous match is the point: every caller is about to act on the
// single key the operator meant, and guessing which of several they had in mind
// is worse than making them narrow the filter.
func findOneKey(library lib.KeyLibrary, filter Filter) (data.Key, error) {
	var keys []data.Key

	for k := range library.ListMatching(keyFilter(filter)) {
		output.Debug("Found matching key ", k)
		keys = append(keys, k)
	}

	switch len(keys) {
	case 0:
		return nil, errors.New("No keys found")
	case 1:
		return keys[0], nil
	default:
		return nil, fmt.Errorf("only a single key result permitted; %d matched", len(keys))
	}
}

// findKey is findOneKey narrowed to an SSH key, which is all `add-id` can work
// with: extra identifiers exist to correlate AWS key-pair fingerprints with an
// SSH key locksmith already holds.
func findKey(library lib.KeyLibrary, filter Filter) (*data.SSHKey, error) {
	k, err := findOneKey(library, filter)
	if err != nil {
		return nil, err
	}

	sshKey, ok := k.(*data.SSHKey)
	if !ok {
		return nil, errors.New("Can only add extra IDs to SSHKey")
	}
	return sshKey, nil
}
