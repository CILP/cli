package spacequota_test

import (
	"github.com/cloudfoundry/cli/cf/api/apifakes"
	"github.com/cloudfoundry/cli/cf/api/spacequotas/spacequotasfakes"
	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/cloudfoundry/cli/cf/requirements"
	"github.com/cloudfoundry/cli/cf/requirements/requirementsfakes"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"

	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("set-space-quota command", func() {
	var (
		ui                  *testterm.FakeUI
		spaceRepo           *apifakes.FakeSpaceRepository
		quotaRepo           *spacequotasfakes.FakeSpaceQuotaRepository
		requirementsFactory *requirementsfakes.FakeFactory
		configRepo          coreconfig.Repository
		deps                commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.Config = configRepo
		deps.RepoLocator = deps.RepoLocator.SetSpaceQuotaRepository(quotaRepo)
		deps.RepoLocator = deps.RepoLocator.SetSpaceRepository(spaceRepo)
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("set-space-quota").SetDependency(deps, pluginCall))
	}

	BeforeEach(func() {
		ui = &testterm.FakeUI{}
		configRepo = testconfig.NewRepositoryWithDefaults()
		spaceRepo = new(apifakes.FakeSpaceRepository)
		quotaRepo = new(spacequotasfakes.FakeSpaceQuotaRepository)
		requirementsFactory = new(requirementsfakes.FakeFactory)
		requirementsFactory.NewLoginRequirementReturns(requirements.Passing{})
	})

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("set-space-quota", args, requirementsFactory, updateCommandDependency, false, ui)
	}

	Describe("requirements", func() {
		It("requires the user to be logged in", func() {
			requirementsFactory.NewLoginRequirementReturns(requirements.Failing{Message: "not logged in"})
			Expect(runCommand("space", "space-quota")).ToNot(HavePassedRequirements())
		})

		It("requires the user to target an org", func() {
			orgReq := new(requirementsfakes.FakeTargetedOrgRequirement)
			orgReq.ExecuteReturns(errors.New("not targeting org"))
			requirementsFactory.NewTargetedOrgRequirementReturns(orgReq)
			Expect(runCommand("space", "space-quota")).ToNot(HavePassedRequirements())
		})

		It("fails with usage if the user does not provide a quota and space", func() {
			requirementsFactory.NewLoginRequirementReturns(requirements.Passing{})
			requirementsFactory.NewTargetedOrgRequirementReturns(new(requirementsfakes.FakeTargetedOrgRequirement))
			runCommand()
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Incorrect Usage", "Requires", "arguments"},
			))
		})
	})

	Context("when logged in", func() {
		JustBeforeEach(func() {
			requirementsFactory.NewLoginRequirementReturns(requirements.Passing{})
			requirementsFactory.NewTargetedOrgRequirementReturns(new(requirementsfakes.FakeTargetedOrgRequirement))
			runCommand("my-space", "quota-name")
		})

		Context("when the space and quota both exist", func() {
			BeforeEach(func() {
				quotaRepo.FindByNameReturns(
					models.SpaceQuota{
						Name:                    "quota-name",
						GUID:                    "quota-guid",
						MemoryLimit:             1024,
						InstanceMemoryLimit:     512,
						RoutesLimit:             111,
						ServicesLimit:           222,
						NonBasicServicesAllowed: true,
						OrgGUID:                 "my-org-guid",
					}, nil)

				spaceRepo.FindByNameReturns(
					models.Space{
						SpaceFields: models.SpaceFields{
							Name: "my-space",
							GUID: "my-space-guid",
						},
						SpaceQuotaGUID: "",
					}, nil)
			})

			Context("when the space quota was not previously assigned to a space", func() {
				It("associates the provided space with the provided space quota", func() {
					spaceGUID, quotaGUID := quotaRepo.AssociateSpaceWithQuotaArgsForCall(0)

					Expect(spaceGUID).To(Equal("my-space-guid"))
					Expect(quotaGUID).To(Equal("quota-guid"))
					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"Assigning space quota", "to space", "my-user"},
						[]string{"OK"},
					))
				})
			})

			Context("when the space quota was previously assigned to a space", func() {
				BeforeEach(func() {
					spaceRepo.FindByNameReturns(
						models.Space{
							SpaceFields: models.SpaceFields{
								Name: "my-space",
								GUID: "my-space-guid",
							},
							SpaceQuotaGUID: "another-quota",
						}, nil)
				})

				It("warns the user that the operation was not performed", func() {
					Expect(quotaRepo.UpdateCallCount()).To(Equal(0))
					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"Assigning space quota", "to space", "my-user"},
						[]string{"FAILED"},
						[]string{"This space already has an assigned space quota."},
					))
				})
			})
		})

		Context("when an error occurs fetching the space", func() {
			BeforeEach(func() {
				spaceRepo.FindByNameReturns(models.Space{}, errors.New("space-repo-err"))
			})

			It("prints an error", func() {
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Assigning space quota", "to space", "my-user"},
					[]string{"FAILED"},
					[]string{"space-repo-err"},
				))
			})
		})

		Context("when an error occurs fetching the quota", func() {
			BeforeEach(func() {
				spaceRepo.FindByNameReturns(
					models.Space{
						SpaceFields: models.SpaceFields{
							Name: "my-space",
							GUID: "my-space-guid",
						},
						SpaceQuotaGUID: "",
					}, nil)

				quotaRepo.FindByNameReturns(models.SpaceQuota{}, errors.New("I can't find my quota name!"))
			})

			It("prints an error", func() {
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Assigning space quota", "to space", "my-user"},
					[]string{"FAILED"},
					[]string{"I can't find my quota name!"},
				))
			})
		})
	})
})
