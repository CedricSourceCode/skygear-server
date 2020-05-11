package flows

import (
	"encoding/json"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/challenge"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/identity/anonymous"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/interaction"
	"github.com/skygeario/skygear-server/pkg/auth/model"
	"github.com/skygeario/skygear-server/pkg/core/authn"
)

type AnonymousIdentityProvider interface {
	ParseRequest(requestJWT string) (*anonymous.Identity, *anonymous.Request, error)
}

type ChallengeProvider interface {
	Consume(token string) (*challenge.Purpose, error)
}

type AnonymousFlow struct {
	Interactions InteractionProvider
	Anonymous    AnonymousIdentityProvider
	Challenges   ChallengeProvider
}

func (f *AnonymousFlow) Authenticate(requestJWT string, clientID string) (*authn.Attrs, error) {
	identity, request, err := f.Anonymous.ParseRequest(requestJWT)
	if err != nil || request.Action != anonymous.RequestActionAuth {
		return nil, interaction.ErrInvalidCredentials
	}

	// Verify challenge token
	purpose, err := f.Challenges.Consume(request.Challenge)
	if err != nil || *purpose != challenge.PurposeAnonymousRequest {
		return nil, interaction.ErrInvalidCredentials
	}

	var keyID string
	if identity != nil {
		keyID = identity.KeyID
	} else {
		// Sign up if identity does not exist
		jwk, err := json.Marshal(request.Key)
		if err != nil {
			return nil, interaction.ErrInvalidCredentials
		}

		i, err := f.Interactions.NewInteractionSignup(&interaction.IntentSignup{
			Identity: interaction.IdentitySpec{
				Type: authn.IdentityTypeAnonymous,
				Claims: map[string]interface{}{
					interaction.IdentityClaimAnonymousKeyID: request.Key.KeyID(),
					interaction.IdentityClaimAnonymousKey:   string(jwk),
				},
			},
			OnUserDuplicate: model.OnUserDuplicateAbort,
		}, clientID)
		if err != nil {
			return nil, err
		}
		s, err := f.Interactions.GetInteractionState(i)
		if err != nil {
			return nil, err
		}
		if s.CurrentStep().Step != interaction.StepCommit {
			panic("interaction_flow_anonymous: unexpected interaction state")
		}
		_, err = f.Interactions.Commit(i)
		if err != nil {
			return nil, err
		}

		keyID = request.Key.KeyID()
	}

	// Login after ensuring user & identity exists
	i, err := f.Interactions.NewInteractionLogin(&interaction.IntentLogin{
		Identity: interaction.IdentitySpec{
			Type: authn.IdentityTypeAnonymous,
			Claims: map[string]interface{}{
				interaction.IdentityClaimAnonymousKeyID: keyID,
			},
		},
	}, clientID)
	if err != nil {
		return nil, err
	}
	s, err := f.Interactions.GetInteractionState(i)
	if err != nil {
		return nil, err
	}
	if s.CurrentStep().Step != interaction.StepCommit {
		panic("interaction_flow_anonymous: unexpected interaction state")
	}
	attrs, err := f.Interactions.Commit(i)
	if err != nil {
		return nil, err
	}

	return attrs, nil
}
