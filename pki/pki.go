package pki

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

// Pki pki info
type Pki struct {
	PublicKeyRing string
	SecretKeyRing string
	PgpKeyName    string
	PublicKey     *openpgp.Entity
	SecretKey     *openpgp.Entity
	PubRing       *openpgp.EntityList
	SecRing       *openpgp.EntityList
}

// New returns a pki object
func New(pgpKeyName string, publicKeyRing string, secretKeyRing string) Pki {
	var err error

	p := Pki{publicKeyRing, secretKeyRing, pgpKeyName, nil, nil, nil, nil}
	publicKeyRing, err = p.ExpandTilde(p.PublicKeyRing)
	if err != nil {
		logger.Fatal("cannot expand public key ring path: ", err)
	}
	p.PublicKeyRing = publicKeyRing
	p.PubRing, err = p.setKeyRing(p.PublicKeyRing)
	if err != nil {
		logger.Fatalf("Pki: %s", err)
	}

	secKeyRing, err := p.ExpandTilde(p.SecretKeyRing)
	if err != nil {
		logger.Fatal("cannot expand secret key ring path: ", err)
	}
	p.SecretKeyRing = secKeyRing
	p.SecRing, err = p.setKeyRing(p.SecretKeyRing)
	if err != nil {
		logger.Warnf("Pki: %s", err)
	}

	// TODO: Something is goofy here sometimes when getting a key to decrypt
	if p.SecRing != nil {
		p.SecretKey = p.GetKeyByID(p.SecRing, p.PgpKeyName)
	}
	p.PublicKey = p.GetKeyByID(p.PubRing, p.PgpKeyName)
	if p.PublicKey == nil {
		logger.Fatalf("unable to find key '%s' in %s", p.PgpKeyName, p.PublicKeyRing)
	}

	return p
}

func (p *Pki) setKeyRing(keyRingPath string) (*openpgp.EntityList, error) {
	keyRing, err := p.ExpandTilde(keyRingPath)
	if err != nil {
		return nil, fmt.Errorf("error reading secring: %s", err)
	}
	keyRingFile, err := os.Open(filepath.Clean(keyRing))
	if err != nil {
		return nil, fmt.Errorf("unable to open key ring: %s", err)
	}
	ring, err := openpgp.ReadKeyRing(keyRingFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read private keys: %s", err)
	} else if ring == nil {
		return nil, fmt.Errorf("%s is empty", p.SecretKeyRing)
	}
	if err = keyRingFile.Close(); err != nil {
		return &ring, fmt.Errorf("error closing secring: %s", err)
	}

	return &ring, nil
}

// EncryptSecret returns encrypted plainText
func (p *Pki) EncryptSecret(plainText string) (string, error) {
	var memBuffer bytes.Buffer

	hints := openpgp.FileHints{IsBinary: false, ModTime: time.Time{}}
	writer := bufio.NewWriter(&memBuffer)
	w, err := armor.Encode(writer, "PGP MESSAGE", nil)
	if err != nil {
		return plainText, fmt.Errorf("encode error: %s", err)
	}

	plainFile, err := openpgp.Encrypt(w, []*openpgp.Entity{p.PublicKey}, nil, &hints, nil)
	if err != nil {
		return plainText, fmt.Errorf("encryption error: %s", err)
	}

	if _, err = fmt.Fprintf(plainFile, "%s", plainText); err != nil {
		return plainText, fmt.Errorf("encryption error: %s", err)
	}

	if err = plainFile.Close(); err != nil {
		return plainText, fmt.Errorf("encryption error: %s", err)
	}
	if err = w.Close(); err != nil {
		return plainText, fmt.Errorf("encryption error: %s", err)
	}
	if err = writer.Flush(); err != nil {
		return plainText, fmt.Errorf("encryption error: %s", err)
	}

	return memBuffer.String(), nil
}

// DecryptSecret returns decrypted cipherText
func (p *Pki) DecryptSecret(cipherText string) (plainText string, err error) {
	if p.SecRing == nil {
		return cipherText, fmt.Errorf("no secring set")
	}

	decbuf := bytes.NewBuffer([]byte(cipherText))
	block, err := armor.Decode(decbuf)
	if err != nil {
		return cipherText, fmt.Errorf("Decode error: %s", err)
	}
	if block.Type != "PGP MESSAGE" {
		return cipherText, fmt.Errorf("block type is not PGP MESSAGE: %s", err)
	}

	md, err := openpgp.ReadMessage(block.Body, p.SecRing, nil, nil)
	if err != nil {
		return cipherText, fmt.Errorf("unable to read PGP message: %s", err)
	}

	body, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return cipherText, fmt.Errorf("unable to read message body: %s", err)
	}

	return string(body), err
}

// GetKeyByID returns a keyring by the given ID
func (p *Pki) GetKeyByID(keyring *openpgp.EntityList, id interface{}) *openpgp.Entity {
	for _, entity := range *keyring {
		if entity.PrivateKey != nil && entity.PrivateKey.KeyIdString() == id.(string) {
			return entity
		}
		if entity.PrimaryKey != nil && entity.PrimaryKey.KeyIdString() == id.(string) {
			return entity
		}

		if checkIdentities(id.(string), entity) {
			return entity
		}
	}

	return nil
}

func checkIdentities(id string, entity *openpgp.Entity) bool {
	for _, ident := range entity.Identities {
		if id == ident.Name {
			return true
		}
		if id == ident.UserId.Email {
			return true
		}
		if id == ident.UserId.Name {
			return true
		}
		if id == ident.UserId.Id {
			return true
		}
	}

	return false
}

// ExpandTilde does exactly what it says on the tin
func (p *Pki) ExpandTilde(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}

// KeyUsedForEncryptedFile gets the key used to encrypt a file
func (p *Pki) KeyUsedForEncryptedFile(file string) (string, error) {
	filePath, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}

	in, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return "", err
	}

	block, err := armor.Decode(in)
	if err != nil {
		return "", err
	}

	if block.Type != "PGP MESSAGE" {
		return "", fmt.Errorf("error decoding private key")
	}
	md, err := openpgp.ReadMessage(block.Body, p.SecRing, nil, nil)
	if err != nil {
		return "", fmt.Errorf("unable to read PGP message: %s", err)
	}

	for index := 0; index < len(md.EncryptedToKeyIds); index++ {
		id := md.EncryptedToKeyIds[index]
		keyStr := p.keyStringForID(id)
		if keyStr != "" {
			return keyStr, nil
		}
	}

	return "", fmt.Errorf("unable to find key for ids used")
}

func (p *Pki) keyStringForID(id uint64) string {
	keys := p.SecRing.KeysById(id, nil)
	if len(keys) > 0 {
		for n := 0; n < len(keys); n++ {
			key := keys[n]
			if key.Entity != nil {
				for k := range key.Entity.Identities {
					// return the first valid key
					return fmt.Sprintf("%X: %s\n", id, k)
				}
			}
		}
	}
	return ""
}
