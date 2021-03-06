package oob

import (
	"context"
	"errors"
	"sort"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/authenticator"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/urlprefix"
	taskspec "github.com/skygeario/skygear-server/pkg/auth/task/spec"
	"github.com/skygeario/skygear-server/pkg/core/async"
	"github.com/skygeario/skygear-server/pkg/core/authn"
	"github.com/skygeario/skygear-server/pkg/core/config"
	"github.com/skygeario/skygear-server/pkg/core/intl"
	"github.com/skygeario/skygear-server/pkg/core/mail"
	"github.com/skygeario/skygear-server/pkg/core/sms"
	"github.com/skygeario/skygear-server/pkg/core/template"
	"github.com/skygeario/skygear-server/pkg/core/time"
	"github.com/skygeario/skygear-server/pkg/core/uuid"
)

type Provider struct {
	Context                   context.Context
	LocalizationConfiguration *config.LocalizationConfiguration
	MetadataConfiguration     config.AuthUIMetadataConfiguration
	SMSMessageConfiguration   config.SMSMessageConfiguration
	EmailMessageConfiguration config.EmailMessageConfiguration
	Config                    *config.AuthenticatorOOBConfiguration
	Store                     *Store
	TemplateEngine            *template.Engine
	URLPrefixProvider         urlprefix.Provider
	TaskQueue                 async.Queue
	Time                      time.Provider
}

func (p *Provider) Get(userID string, id string) (*Authenticator, error) {
	return p.Store.Get(userID, id)
}

func (p *Provider) GetByChannel(userID string, channel authn.AuthenticatorOOBChannel, phone string, email string) (*Authenticator, error) {
	return p.Store.GetByChannel(userID, channel, phone, email)
}

func (p *Provider) Delete(a *Authenticator) error {
	return p.Store.Delete(a.ID)
}

func (p *Provider) List(userID string) ([]*Authenticator, error) {
	authenticators, err := p.Store.List(userID)
	if err != nil {
		return nil, err
	}

	sortAuthenticators(authenticators)
	return authenticators, nil
}

func (p *Provider) New(userID string, channel authn.AuthenticatorOOBChannel, phone string, email string) *Authenticator {
	a := &Authenticator{
		ID:      uuid.New(),
		UserID:  userID,
		Channel: channel,
		Phone:   phone,
		Email:   email,
	}
	return a
}

func (p *Provider) Create(a *Authenticator) error {
	_, err := p.Store.GetByChannel(a.UserID, a.Channel, a.Phone, a.Email)
	if err == nil {
		return authenticator.ErrAuthenticatorAlreadyExists
	} else if !errors.Is(err, authenticator.ErrAuthenticatorNotFound) {
		return err
	}

	now := p.Time.NowUTC()
	a.CreatedAt = now

	return p.Store.Create(a)
}

func (p *Provider) Authenticate(expectedCode string, code string) error {
	ok := VerifyCode(expectedCode, code)
	if !ok {
		return errors.New("invalid bearer token")
	}
	return nil
}

func (p *Provider) GenerateCode() string {
	return GenerateCode()
}

type SendCodeOptions struct {
	Channel string
	Email   string
	Phone   string
	Code    string
}

func (p *Provider) SendCode(opts SendCodeOptions) (err error) {
	urlPrefix := p.URLPrefixProvider.Value()
	email := opts.Email
	phone := opts.Phone
	channel := opts.Channel
	code := opts.Code

	data := map[string]interface{}{
		"email": email,
		"phone": phone,
		"code":  code,
		"host":  urlPrefix.Host,
	}

	preferredLanguageTags := intl.GetPreferredLanguageTags(p.Context)
	data["appname"] = intl.LocalizeJSONObject(preferredLanguageTags, intl.Fallback(p.LocalizationConfiguration.FallbackLanguage), p.MetadataConfiguration, "app_name")

	switch channel {
	case string(authn.AuthenticatorOOBChannelEmail):
		return p.SendEmail(email, data)
	case string(authn.AuthenticatorOOBChannelSMS):
		return p.SendSMS(phone, data)
	default:
		panic("expected OOB channel: " + string(channel))
	}
}

func (p *Provider) SendEmail(email string, data map[string]interface{}) (err error) {
	textBody, err := p.TemplateEngine.RenderTemplate(
		TemplateItemTypeOOBCodeEmailTXT,
		data,
		template.ResolveOptions{},
	)
	if err != nil {
		return
	}

	htmlBody, err := p.TemplateEngine.RenderTemplate(
		TemplateItemTypeOOBCodeEmailHTML,
		data,
		template.ResolveOptions{},
	)
	if err != nil {
		return
	}

	p.TaskQueue.Enqueue(async.TaskSpec{
		Name: taskspec.SendMessagesTaskName,
		Param: taskspec.SendMessagesTaskParam{
			EmailMessages: []mail.SendOptions{
				{
					MessageConfig: config.NewEmailMessageConfiguration(
						p.EmailMessageConfiguration,
						p.Config.Email.Message,
					),
					Recipient: email,
					TextBody:  textBody,
					HTMLBody:  htmlBody,
				},
			},
		},
	})

	return
}

func (p *Provider) SendSMS(phone string, data map[string]interface{}) (err error) {
	body, err := p.TemplateEngine.RenderTemplate(
		TemplateItemTypeOOBCodeSMSTXT,
		data,
		template.ResolveOptions{},
	)
	if err != nil {
		return
	}

	p.TaskQueue.Enqueue(async.TaskSpec{
		Name: taskspec.SendMessagesTaskName,
		Param: taskspec.SendMessagesTaskParam{
			SMSMessages: []sms.SendOptions{
				{
					MessageConfig: config.NewSMSMessageConfiguration(
						p.SMSMessageConfiguration,
						p.Config.SMS.Message,
					),
					To:   phone,
					Body: body,
				},
			},
		},
	})

	return
}
func sortAuthenticators(as []*Authenticator) {
	sort.Slice(as, func(i, j int) bool {
		return as[i].CreatedAt.Before(as[j].CreatedAt)
	})
}
