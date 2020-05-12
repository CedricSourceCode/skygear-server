package interaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/authenticator/oob"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/identity"
	"github.com/skygeario/skygear-server/pkg/core/authn"
	"github.com/skygeario/skygear-server/pkg/core/skyerr"
)

func (p *Provider) PerformAction(i *Interaction, step Step, action Action) error {
	state, err := p.GetInteractionState(i)
	if err != nil {
		return err
	}

	var stepState *StepState
	for _, s := range state.Steps {
		if s.Step == step {
			stepState = &s
			break
		}
	}
	if stepState == nil {
		return ErrInvalidStep
	}

	switch intent := i.Intent.(type) {
	case *IntentLogin:
		return p.performActionLogin(i, intent, stepState, state, action)
	case *IntentSignup:
		return p.performActionSignup(i, intent, stepState, state, action)
	case *IntentAddIdentity:
		return p.performActionAddIdentity(i, intent, stepState, state, action)
	case *IntentUpdateIdentity:
		return p.performActionUpdateIdentity(i, intent, stepState, state, action)
	}
	panic(fmt.Sprintf("interaction: unknown intent type %T", i.Intent))
}

func (p *Provider) performActionLogin(i *Interaction, intent *IntentLogin, step *StepState, s *State, action Action) error {
	switch step.Step {
	case StepAuthenticatePrimary, StepAuthenticateSecondary:
		switch action := action.(type) {
		case *ActionAuthenticate:
			authen, err := p.doAuthenticate(i, step, &i.State, intent.Identity, action.Authenticator, action.Secret)
			if skyerr.IsAPIError(err) {
				i.Error = skyerr.AsAPIError(err)
				return nil
			} else if err != nil {
				return err
			}

			ar := authen.ToRef()
			if step.Step == StepAuthenticatePrimary {
				i.PrimaryAuthenticator = &ar
				i.SecondaryAuthenticator = nil
			} else {
				i.SecondaryAuthenticator = &ar
			}
			i.Error = nil
			return nil

		case *ActionTriggerOOBAuthenticator:
			err := p.doTriggerOOB(i, action)
			if err != nil {
				return err
			}
			return nil
		default:
			panic(fmt.Sprintf("interaction_login: unhandled authenticate action %T", action))
		}

	case StepSetupSecondaryAuthenticator:
		// TODO(interaction): setup secondary authenticator

	case StepCommit:
		// TODO(interaction): allow setup bearer token

	}
	panic("interaction_login: unhandled step " + step.Step)
}

func (p *Provider) performActionSignup(i *Interaction, intent *IntentSignup, step *StepState, s *State, action Action) error {
	switch step.Step {
	case StepSetupPrimaryAuthenticator:
		return p.setupPrimaryAuthenticator(i, step, s, action)
	}
	panic("interaction_signup: unhandled step " + step.Step)
}

func (p *Provider) performActionAddIdentity(i *Interaction, intent *IntentAddIdentity, step *StepState, s *State, action Action) error {
	switch step.Step {
	case StepSetupPrimaryAuthenticator:
		return p.setupPrimaryAuthenticator(i, step, s, action)
	}
	panic("interaction_add_identity: unhandled step " + step.Step)
}

func (p *Provider) performActionUpdateIdentity(i *Interaction, intent *IntentUpdateIdentity, step *StepState, s *State, action Action) error {
	switch step.Step {
	case StepSetupPrimaryAuthenticator:
		// setup primary authenticator for updated identity
		return p.setupPrimaryAuthenticator(i, step, s, action)
	}
	panic("interaction_add_identity: unhandled step " + step.Step)
}

func (p *Provider) setupPrimaryAuthenticator(i *Interaction, step *StepState, s *State, action Action) error {
	switch action := action.(type) {
	case *ActionSetupAuthenticator:
		authen, err := p.setupAuthenticator(i, step, &i.State, action.Authenticator, action.Secret)
		if skyerr.IsAPIError(err) {
			i.Error = skyerr.AsAPIError(err)
			return nil
		} else if err != nil {
			return err
		}

		ar := authen.ToRef()
		i.PrimaryAuthenticator = &ar
		i.Error = nil
		return nil

	case *ActionTriggerOOBAuthenticator:
		err := p.doTriggerOOB(i, action)
		if err != nil {
			return err
		}
		return nil
	default:
		panic(fmt.Sprintf("interaction_signup: unhandled authenticate action %T", action))
	}
}

func (p *Provider) doAuthenticate(i *Interaction, step *StepState, astate *map[string]string, is identity.Spec, as AuthenticatorSpec, secret string) (*AuthenticatorInfo, error) {
	userID, iden, err := p.Identity.GetByClaims(is.Type, is.Claims)
	if errors.Is(err, identity.ErrIdentityNotFound) {
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	authen, err := p.Authenticator.Authenticate(userID, as, astate, secret)
	if err != nil {
		return nil, err
	}

	i.UserID = userID
	ir := iden.ToRef()
	i.Identity = &ir
	i.State = nil
	return authen, nil
}

func (p *Provider) setupAuthenticator(i *Interaction, step *StepState, astate *map[string]string, as AuthenticatorSpec, secret string) (*AuthenticatorInfo, error) {
	ok := false
	for _, aa := range step.AvailableAuthenticators {
		if aa.Type == as.Type {
			ok = true
			break
		}
	}
	if !ok {
		// Authenticator is not available for current step, reject it
		return nil, ErrInvalidAction
	}

	switch as.Type {
	case authn.AuthenticatorTypePassword:
		// Nothing special needs to be done
		break
	case authn.AuthenticatorTypeOOB:
		// Ignoring the first return value because it is always nil.
		_, err := p.Authenticator.Authenticate(i.UserID, as, astate, secret)
		if err != nil {
			return nil, err
		}
	default:
		panic("interaction_signup: setup up unexpected authenticator type: " + as.Type)
	}

	ais, err := p.Authenticator.New(i.UserID, as, secret)
	if err != nil {
		return nil, err
	}
	i.NewAuthenticators = append(i.NewAuthenticators, ais...)
	i.State = nil
	return ais[0], nil
}

func (p *Provider) doTriggerOOB(i *Interaction, action *ActionTriggerOOBAuthenticator) (err error) {
	spec := action.Authenticator

	if spec.Type != authn.AuthenticatorTypeOOB {
		panic("interaction: unexpected ActionTriggerOOBAuthenticator.Authenticator.Type: " + spec.Type)
	}

	now := p.Time.NowUTC()
	nowBytes, err := now.MarshalText()
	if err != nil {
		return
	}
	nowStr := string(nowBytes)

	if i.State == nil {
		i.State = map[string]string{}
	}

	// Rotate the code according to oob.OOBCodeValidDuration
	code := i.State[AuthenticatorStateOOBOTPCode]
	generateTimeStr := i.State[AuthenticatorStateOOBOTPGenerateTime]
	if generateTimeStr == "" {
		code = p.OOB.GenerateCode()
		generateTimeStr = nowStr
	} else {
		var tt time.Time
		err = tt.UnmarshalText([]byte(generateTimeStr))
		if err != nil {
			return
		}

		// Expire
		if tt.Add(oob.OOBCodeValidDuration).Before(now) {
			code = p.OOB.GenerateCode()
			generateTimeStr = nowStr
		}
	}

	// Respect cooldown
	triggerTimeStr := i.State[AuthenticatorStateOOBOTPTriggerTime]
	if triggerTimeStr != "" {
		var tt time.Time
		err = tt.UnmarshalText([]byte(triggerTimeStr))
		if err != nil {
			return
		}

		if tt.Add(oob.OOBCodeSendCooldownSeconds * time.Second).After(now) {
			err = ErrOOBOTPCooldown
			return
		}
	}

	opts := oob.SendCodeOptions{
		Code: code,
	}
	if channel, ok := spec.Props[AuthenticatorPropOOBOTPChannelType].(string); ok {
		opts.Channel = channel
	}
	if email, ok := spec.Props[AuthenticatorPropOOBOTPEmail].(string); ok {
		opts.Email = email
	}
	if phone, ok := spec.Props[AuthenticatorPropOOBOTPPhone].(string); ok {
		opts.Phone = phone
	}

	err = p.OOB.SendCode(opts)
	if err != nil {
		return
	}

	// Perform mutation on interaction at the end.

	// This function can be called by login or signup.
	// In case of signup, the spec does not have an ID yet.
	if id, ok := spec.Props[AuthenticatorPropOOBOTPID].(string); ok {
		i.State[AuthenticatorStateOOBOTPID] = id
	}
	i.State[AuthenticatorStateOOBOTPCode] = code
	i.State[AuthenticatorStateOOBOTPGenerateTime] = generateTimeStr
	i.State[AuthenticatorStateOOBOTPTriggerTime] = nowStr

	return
}
