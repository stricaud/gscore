/*
 * crypt.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A .gscore locked with a password.
 *
 * The container is still a zip, so that everything that finds a .gscore by
 * its zip directory — the PDF prefix, the symbol readers — still finds it.
 * What changes is what the zip holds: a format.json that says how the file
 * is locked, and one entry that is the ordinary container sealed under a
 * key made from the password. AES-256-GCM, so that a wrong password is
 * told apart from a damaged file, and PBKDF2 over SHA-256 with enough
 * rounds that guessing costs something.
 *
 * A locked file carries no PDF in front of it. The picture is the music,
 * and a file that shows it to anyone while asking a password for the notes
 * is locked in name only: a .gscore saved under a password is the sealed
 * container alone, and nothing of the piece is readable without the word.
 */

package gscore

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/symbol"
	"github.com/stricaud/gscore/internal/version"
)

// ErrPasswordNeeded says the file is locked and no password was given.
var ErrPasswordNeeded = errors.New("gscore: the file is protected by a password")

// ErrWrongPassword says the password given does not open the file.
var ErrWrongPassword = errors.New("gscore: that is not the password")

const (
	// encryptedEntry is the one entry a locked container holds besides
	// its format.json: the ordinary container, sealed.
	encryptedEntry = "encrypted/score.zip"
	// encryptedVersion is the container version a locked file claims, so
	// that a reader from before locking existed says the file is newer
	// than it is rather than that it holds no score.
	encryptedVersion = 2
	cipherName       = "AES-256-GCM"
	kdfName          = "PBKDF2-HMAC-SHA256"
	// kdfRounds is how many times the password is hashed into the key.
	// Enough to slow a guess to a crawl, few enough that opening a score
	// takes a moment and not a breath.
	kdfRounds = 200000
)

// encryptionInfo is how a locked file says it is locked, in its format.json.
type encryptionInfo struct {
	Cipher     string `json:"cipher"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
}

// WriteEncrypted saves a score locked under a password: the sealed
// container, with no PDF in front of it.
func WriteEncrypted(w io.Writer, s *model.Score, symbols *symbol.Registry, password string) error {
	if password == "" {
		return errors.New("gscore: a password is needed to protect the file")
	}
	// The ordinary container, whole, is what gets sealed: the music, the
	// symbols and their index, exactly as an unlocked file holds them.
	var inner bytes.Buffer
	if err := write(&inner, s, symbols, usedSymbols(s)); err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("gscore: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, kdfRounds, 32)
	if err != nil {
		return fmt.Errorf("gscore: %w", err)
	}
	sealed, err := seal(key, inner.Bytes())
	if err != nil {
		return fmt.Errorf("gscore: %w", err)
	}

	zw := zip.NewWriter(w)
	info, _ := json.MarshalIndent(formatInfo{
		Format: "gscore", Version: encryptedVersion,
		Generator: "gscore " + version.Version,
		Encryption: &encryptionInfo{
			Cipher: cipherName, KDF: kdfName, Iterations: kdfRounds,
			Salt: base64.StdEncoding.EncodeToString(salt),
		},
	}, "", "  ")
	f, err := zw.Create("META-INF/format.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(append(info, '\n')); err != nil {
		return err
	}
	// Stored, not deflated: cipher text does not compress, and a deflater
	// run over it only spends time finding that out.
	f, err = zw.CreateHeader(&zip.FileHeader{Name: encryptedEntry, Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := f.Write(sealed); err != nil {
		return err
	}
	return zw.Close()
}

// seal encrypts with a fresh nonce in front, and open undoes it.
func seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func open(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, ErrWrongPassword
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		// GCM cannot tell a wrong key from a damaged file; a wrong
		// password is what it nearly always is.
		return nil, ErrWrongPassword
	}
	return plain, nil
}

// openContainer reads the entries of a .gscore, unlocking it where it is
// locked. It hands back the entries and what the container's format.json
// said — the inner one's, for a locked file.
func openContainer(data []byte, password string) (map[string]*zip.File, formatInfo, error) {
	var info formatInfo
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, info, fmt.Errorf("gscore: not a .gscore container: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	if f, ok := files["META-INF/format.json"]; ok {
		if b, err := readAll(f); err == nil {
			_ = json.Unmarshal(b, &info)
		}
	}
	enc := info.Encryption
	if enc == nil {
		return files, info, nil
	}
	if password == "" {
		return nil, info, ErrPasswordNeeded
	}
	if enc.Cipher != cipherName || enc.KDF != kdfName || enc.Iterations <= 0 {
		return nil, info, fmt.Errorf("gscore: the file is locked in a way this version cannot open (%s, %s)", enc.Cipher, enc.KDF)
	}
	salt, err := base64.StdEncoding.DecodeString(enc.Salt)
	if err != nil {
		return nil, info, fmt.Errorf("gscore: the file's lock is damaged: %w", err)
	}
	sf, ok := files[encryptedEntry]
	if !ok {
		return nil, info, errors.New("gscore: the locked container holds nothing")
	}
	sealed, err := readAll(sf)
	if err != nil {
		return nil, info, fmt.Errorf("gscore: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, enc.Iterations, 32)
	if err != nil {
		return nil, info, fmt.Errorf("gscore: %w", err)
	}
	plain, err := open(key, sealed)
	if err != nil {
		return nil, info, err
	}
	// The inner container is an ordinary one, and is read as one. Not
	// with the password: a container locked inside a locked container is
	// nothing this writes, and reading it would be reading forever.
	return openContainer(plain, "")
}
