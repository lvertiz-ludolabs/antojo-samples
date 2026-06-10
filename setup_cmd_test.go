package app

import (
	"context"
	"testing"

	"antojo/api/internal/org/domain"
	orgContactApp "antojo/api/internal/org_contact/app"
	sharedMocks "antojo/api/internal/shared/domain"
	userApp "antojo/api/internal/user/app"
	userDomain "antojo/api/internal/user/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSetupOrgCommand_Execute(t *testing.T) {
	ctx := context.Background()

	orgRepo := domain.NewMockOrgRepositoryPort(t)
	transactor := sharedMocks.NewMockTransactor(t)
	createUserCmd := userApp.NewMockCreateCommandPort(t)
	addContactCmd := orgContactApp.NewMockCreateCommandPort(t)

	input := SetupCommandInput{
		Name:          "My Org",
		Alias:         "my-org",
		AdminPassword: "password123",
		ContactName:   "John Doe",
		ContactEmail:  "john@example.com",
		ContactPhone:  "1234567",
	}

	createdOrg := &domain.Org{
		ID:    1,
		Name:  input.Name,
		Alias: input.Alias,
	}

	orgRepo.EXPECT().
		FindByAlias(ctx, input.Alias).
		Return(nil, nil)

	orgRepo.EXPECT().
		Create(ctx, domain.Org{Name: input.Name, Alias: input.Alias}).
		Return(createdOrg, nil)

	createUserCmd.EXPECT().
		Execute(ctx, createdOrg.ID, mock.MatchedBy(func(i userApp.CreateCommandInput) bool {
			return i.Username == "admin" &&
				i.Name == input.ContactName &&
				i.Role == userDomain.UserRoleAdmin
		})).
		Return(&userApp.UserDTO{}, nil)

	addContactCmd.EXPECT().
		Execute(ctx, createdOrg.ID, orgContactApp.CreateCommandInput{
			Name:     input.ContactName,
			Email:    &input.ContactEmail,
			Phone:    &input.ContactPhone,
			Position: "",
			Main:     true,
		}).
		Return(&orgContactApp.OrgContactDTO{}, nil)

	transactor.EXPECT().
		RunInTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	command := NewSetupCommand(orgRepo, transactor, createUserCmd, addContactCmd)
	dto, err := command.Execute(ctx, input)
	require.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, input.Name, dto.Name)
	assert.Equal(t, input.Alias, dto.Alias)
}

func TestSetupOrgCommand_Execute_Error(t *testing.T) {
	ctx := context.Background()

	orgRepo := domain.NewMockOrgRepositoryPort(t)
	transactor := sharedMocks.NewMockTransactor(t)
	createUserCmd := userApp.NewMockCreateCommandPort(t)
	addContactCmd := orgContactApp.NewMockCreateCommandPort(t)

	input := SetupCommandInput{
		Name:          "My Org",
		Alias:         "my-org",
		AdminPassword: "password123",
		ContactName:   "John Doe",
		ContactEmail:  "john@example.com",
		ContactPhone:  "1234567",
	}

	alreadyCreatedOrg := &domain.Org{
		ID:    1,
		Name:  input.Name,
		Alias: input.Alias,
	}

	orgRepo.EXPECT().
		FindByAlias(ctx, input.Alias).
		Return(alreadyCreatedOrg, nil)

	command := NewSetupCommand(orgRepo, transactor, createUserCmd, addContactCmd)
	_, err := command.Execute(ctx, input)
	require.Error(t, err)
	assert.Error(t, err)
}

func TestSetupOrgCommand_Execute_Error_InvalidAlias(t *testing.T) {
	ctx := context.Background()
	orgRepo := domain.NewMockOrgRepositoryPort(t)
	transactor := sharedMocks.NewMockTransactor(t)
	createUserCmd := userApp.NewMockCreateCommandPort(t)
	addContactCmd := orgContactApp.NewMockCreateCommandPort(t)
	input := SetupCommandInput{
		Name:  "My Org",
		Alias: "admin",
	}

	command := NewSetupCommand(orgRepo, transactor, createUserCmd, addContactCmd)
	_, err := command.Execute(ctx, input)
	require.Error(t, err)
	assert.Error(t, err)
}
