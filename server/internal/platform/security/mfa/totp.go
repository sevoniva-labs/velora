package mfa

import (
	"github.com/pquerna/otp/totp"
)

type TOTP struct{}

func (TOTP) Generate(issuer, account string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: issuer, AccountName: account})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}
func (TOTP) Validate(code, secret string) bool { return totp.Validate(code, secret) }
