package connector

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/id"

	"github.com/lrhodin/imessage/pkg/api"
)

// loginAdapter implements api.LoginProvider using bridgev2 login flows.
type loginAdapter struct {
	connector *IMConnector
}

var _ api.LoginProvider = (*loginAdapter)(nil)

func (a *loginAdapter) GetLoginFlows() []api.LoginFlowInfo {
	flows := a.connector.Bridge.Network.GetLoginFlows()
	result := make([]api.LoginFlowInfo, len(flows))
	for i, f := range flows {
		result[i] = api.LoginFlowInfo{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
		}
	}
	return result
}

func (a *loginAdapter) StartLogin(ctx context.Context, flowID string) (api.LoginSession, error) {
	user, err := a.findAdminUser(ctx)
	if err != nil {
		return nil, err
	}

	login, err := a.connector.Bridge.Network.CreateLogin(ctx, user, flowID)
	if err != nil {
		return nil, fmt.Errorf("failed to create login: %w", err)
	}

	userInput, ok := login.(bridgev2.LoginProcessUserInput)
	if !ok {
		return nil, fmt.Errorf("login flow %q does not support user input", flowID)
	}

	step, err := login.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start login: %w", err)
	}

	return &loginSessionAdapter{process: userInput, step: step}, nil
}

// findAdminUser returns the first admin user, same pattern as login_cli.go.
func (a *loginAdapter) findAdminUser(ctx context.Context) (*bridgev2.User, error) {
	for userID, perm := range a.connector.Bridge.Config.Permissions {
		if perm.Admin {
			user, err := a.connector.Bridge.GetUserByMXID(ctx, id.UserID(userID))
			if err != nil {
				return nil, fmt.Errorf("failed to get admin user: %w", err)
			}
			return user, nil
		}
	}
	return nil, fmt.Errorf("no admin user found in config permissions")
}

// loginSessionAdapter wraps bridgev2.LoginProcessUserInput + LoginStep
// to satisfy api.LoginSession.
type loginSessionAdapter struct {
	process bridgev2.LoginProcessUserInput
	step    *bridgev2.LoginStep
}

var _ api.LoginSession = (*loginSessionAdapter)(nil)

func (s *loginSessionAdapter) StepID() string {
	return s.step.StepID
}

func (s *loginSessionAdapter) Instructions() string {
	return s.step.Instructions
}

func (s *loginSessionAdapter) Fields() []api.LoginField {
	if s.step.UserInputParams == nil {
		return nil
	}
	fields := make([]api.LoginField, len(s.step.UserInputParams.Fields))
	for i, f := range s.step.UserInputParams.Fields {
		fields[i] = api.LoginField{
			ID:      f.ID,
			Name:    f.Name,
			Type:    string(f.Type),
			Options: f.Options,
		}
	}
	return fields
}

func (s *loginSessionAdapter) Submit(ctx context.Context, input map[string]string) (api.LoginSession, *api.LoginComplete, error) {
	nextStep, err := s.process.SubmitUserInput(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	if nextStep.Type == bridgev2.LoginStepTypeComplete {
		loginID := ""
		if nextStep.CompleteParams != nil {
			loginID = string(nextStep.CompleteParams.UserLoginID)
		}
		return nil, &api.LoginComplete{
			LoginID: loginID,
			Message: nextStep.Instructions,
		}, nil
	}

	s.step = nextStep
	return s, nil, nil
}

