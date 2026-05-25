// Package encryption implements authenticated symmetric encryption —
// Laravel's Crypt facade reimagined for Go. A Cipher driver encrypts
// plaintext into a self-describing string and decrypts back; the
// Manager holds a primary cipher plus a versioned key ring so old
// ciphertext stays decryptable through key rotations.
//
//	enc := encryption.MustNew("base64:<32-byte key>")  // aesgcm default
//	tok, _ := enc.EncryptString(ctx, "secret")
//	pt,  _ := enc.DecryptString(ctx, tok)
//
// Encoded output is "v1:<key-id>:<base64(nonce|ciphertext|tag)>",
// so DecryptString can route through the key ring without the
// caller carrying metadata. The Manager rotates keys by registering
// a new key under a fresh ID and flipping the primary pointer; new
// ciphertext uses the new key, old ciphertext still decrypts under
// the prior key.
//
// Laravel mapping:
//
//	Laravel                           | Framework
//	----------------------------------|----------------------------
//	Crypt::encryptString($plain)       | enc.EncryptString(ctx, plain)
//	Crypt::decryptString($cipher)     | enc.DecryptString(ctx, cipher)
//	Crypt::encrypt($any, $serialize)  | enc.Encrypt(ctx, []byte(...))   (callers serialise)
//	APP_KEY rotation                  | Manager.AddKey + Manager.SetPrimary
package encryption
