package httpapi

import (
	"context"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/authn"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
)

type fakeOIDCProtocol struct {
	begin      authn.Authorization
	identity   domain.OIDCIdentity
	completed  bool
	completion domain.OIDCFlow
}

type fakeModelGateway struct {
	response string
	request  modelgateway.Request
}

func (f *fakeModelGateway) Complete(_ context.Context, _ domain.AIProviderConfig, request modelgateway.Request) (string, error) {
	f.request = request
	return f.response, nil
}

func (f *fakeOIDCProtocol) Begin(context.Context, domain.SignInConfig) (authn.Authorization, error) {
	return f.begin, nil
}

func (f *fakeOIDCProtocol) Complete(_ context.Context, _ domain.SignInConfig, code string, flow domain.OIDCFlow) (domain.OIDCIdentity, error) {
	if code != "authorization-code" {
		return domain.OIDCIdentity{}, fmt.Errorf("unexpected authorization code %q", code)
	}
	f.completed = true
	f.completion = flow
	return f.identity, nil
}

func (f *fakeOIDCProtocol) VerifyConfiguration(context.Context, domain.SignInConfig) error {
	return nil
}
