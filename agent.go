package withdrawal_service

import (
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/interfaces/identity_iface"
)

type v2ImporterAgent struct {
	*authorization.JwtIdentity
}

func NewV2ImporterAgent(identity *authorization.JwtIdentity) *v2ImporterAgent {
	identity.UserAgent = identity_iface.ImporterAgent
	return &v2ImporterAgent{
		JwtIdentity: identity,
	}
}
