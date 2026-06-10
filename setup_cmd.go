package app

import (
	"context"
	"net/http"

	"antojo/api/internal/org/domain"
	orgContactApp "antojo/api/internal/org_contact/app"
	"antojo/api/internal/shared"
	sharedDomain "antojo/api/internal/shared/domain"
	"antojo/api/internal/shared/utils"
	userApp "antojo/api/internal/user/app"
	userDomain "antojo/api/internal/user/domain"
)

//go:generate mockery
type SetupCommandPort interface {
	Execute(ctx context.Context, input SetupCommandInput) (*OrgDTO, error)
}

type SetupCommand struct {
	repo                 domain.OrgRepositoryPort
	transactor           sharedDomain.Transactor
	createUserCommand    userApp.CreateCommandPort
	createContactCommand orgContactApp.CreateCommandPort
}

func NewSetupCommand(
	repo domain.OrgRepositoryPort,
	transactor sharedDomain.Transactor,
	createUserCommand userApp.CreateCommandPort,
	createContactCommand orgContactApp.CreateCommandPort,
) *SetupCommand {
	return &SetupCommand{
		repo:                 repo,
		transactor:           transactor,
		createUserCommand:    createUserCommand,
		createContactCommand: createContactCommand,
	}
}

func (c *SetupCommand) Execute(ctx context.Context, input SetupCommandInput) (*OrgDTO, error) {
	var dto *OrgDTO

	if !utils.IsValidAlias(input.Alias) {
		return nil, shared.NewAppError(http.StatusBadRequest, "Invalid alias", "invalidAlias")
	}

	existingOrg, err := c.repo.FindByAlias(ctx, input.Alias)
	if err != nil {
		return nil, err
	}
	if existingOrg != nil {
		return nil, shared.NewAppError(http.StatusBadRequest, "An organization with this alias already exists", "aliasAlreadyExists")
	}

	err = c.transactor.RunInTx(ctx, func(ctx context.Context) error {
		org := domain.Org{
			Name:  input.Name,
			Alias: input.Alias,
		}
		createdOrg, err := c.repo.Create(ctx, org)
		if err != nil {
			return err
		}
		dto = NewOrgDTO(createdOrg)

		createUserCommandInput := userApp.CreateCommandInput{
			Username: "admin",
			Name:     input.ContactName,
			Password: input.AdminPassword,
			Role:     userDomain.UserRoleAdmin,
		}

		_, err = c.createUserCommand.Execute(ctx, createdOrg.ID, createUserCommandInput)
		if err != nil {
			return err
		}

		addContactCommandInput := orgContactApp.CreateCommandInput{
			Name:     input.ContactName,
			Email:    &input.ContactEmail,
			Phone:    &input.ContactPhone,
			Position: "",
			Main:     true,
		}
		_, err = c.createContactCommand.Execute(ctx, createdOrg.ID, addContactCommandInput)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return dto, nil
}
