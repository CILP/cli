package route_test

import (
	"github.com/cloudfoundry/cli/cf/api/apifakes"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/cloudfoundry/cli/flags"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"

	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/commands/route"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("delete-orphaned-routes command", func() {
	var (
		ui         *testterm.FakeUI
		routeRepo  *apifakes.FakeRouteRepository
		configRepo coreconfig.Repository
		reqFactory *testreq.FakeReqFactory
		deps       commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.RepoLocator = deps.RepoLocator.SetRouteRepository(routeRepo)
		deps.Config = configRepo
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("delete-orphaned-routes").SetDependency(deps, pluginCall))
	}

	callDeleteOrphanedRoutes := func(confirmation string, args []string, reqFactory *testreq.FakeReqFactory, routeRepo *apifakes.FakeRouteRepository) (*testterm.FakeUI, bool) {
		ui = &testterm.FakeUI{Inputs: []string{confirmation}}
		configRepo = testconfig.NewRepositoryWithDefaults()
		passed := testcmd.RunCLICommand("delete-orphaned-routes", args, reqFactory, updateCommandDependency, false, ui)

		return ui, passed
	}

	BeforeEach(func() {
		routeRepo = new(apifakes.FakeRouteRepository)
		reqFactory = &testreq.FakeReqFactory{}
	})

	It("fails requirements when not logged in", func() {
		_, passed := callDeleteOrphanedRoutes("y", []string{}, reqFactory, routeRepo)
		Expect(passed).To(BeFalse())
	})

	Context("when arguments are provided", func() {
		var cmd commandregistry.Command
		var flagContext flags.FlagContext

		BeforeEach(func() {
			cmd = &route.DeleteOrphanedRoutes{}
			cmd.SetDependency(deps, false)
			flagContext = flags.NewFlagContext(cmd.MetaData().Flags)
		})

		It("should fail with usage", func() {
			flagContext.Parse("blahblah")

			reqs := cmd.Requirements(reqFactory, flagContext)

			err := testcmd.RunRequirements(reqs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Incorrect Usage"))
			Expect(err.Error()).To(ContainSubstring("No argument required"))
		})
	})

	Context("when logged in successfully", func() {

		BeforeEach(func() {
			reqFactory.LoginSuccess = true
		})

		It("passes requirements when logged in", func() {
			_, passed := callDeleteOrphanedRoutes("y", []string{}, reqFactory, routeRepo)
			Expect(passed).To(BeTrue())
		})

		It("passes when confirmation is provided", func() {
			var ui *testterm.FakeUI
			domain := models.DomainFields{Name: "example.com"}
			domain2 := models.DomainFields{Name: "cookieclicker.co"}

			app1 := models.ApplicationFields{Name: "dora"}

			routeRepo.ListRoutesStub = func(cb func(models.Route) bool) error {
				route := models.Route{}
				route.GUID = "route1-guid"
				route.Host = "hostname-1"
				route.Domain = domain
				route.Apps = []models.ApplicationFields{app1}

				route2 := models.Route{}
				route2.GUID = "route2-guid"
				route2.Host = "hostname-2"
				route2.Domain = domain2

				cb(route)
				cb(route2)

				return nil
			}

			ui, _ = callDeleteOrphanedRoutes("y", []string{}, reqFactory, routeRepo)

			Expect(ui.Prompts).To(ContainSubstrings(
				[]string{"Really delete orphaned routes"},
			))

			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Deleting route", "hostname-2.cookieclicker.co"},
				[]string{"OK"},
			))

			Expect(routeRepo.DeleteCallCount()).To(Equal(1))
			Expect(routeRepo.DeleteArgsForCall(0)).To(Equal("route2-guid"))
		})

		It("passes when the force flag is used", func() {
			var ui *testterm.FakeUI

			routeRepo.ListRoutesStub = func(cb func(models.Route) bool) error {
				route := models.Route{}
				route.Host = "hostname-1"
				route.Domain = models.DomainFields{Name: "example.com"}
				route.Apps = []models.ApplicationFields{
					{
						Name: "dora",
					},
				}

				route2 := models.Route{}
				route2.GUID = "route2-guid"
				route2.Host = "hostname-2"
				route2.Domain = models.DomainFields{Name: "cookieclicker.co"}

				cb(route)
				cb(route2)

				return nil
			}

			ui, _ = callDeleteOrphanedRoutes("", []string{"-f"}, reqFactory, routeRepo)

			Expect(len(ui.Prompts)).To(Equal(0))

			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Deleting route", "hostname-2.cookieclicker.co"},
				[]string{"OK"},
			))
			Expect(routeRepo.DeleteCallCount()).To(Equal(1))
			Expect(routeRepo.DeleteArgsForCall(0)).To(Equal("route2-guid"))
		})
	})
})
