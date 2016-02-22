package client

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	registrytypes "github.com/docker/engine-api/types/registry"
)

// CmdSearch searches the Docker Hub for images.
//
// Usage: docker search [OPTIONS] TERM
func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := Cli.Subcmd("search", []string{"TERM"}, Cli.DockerCommands["search"].Description, true)
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")
	//	automated := cmd.Bool([]string{"-automated"}, false, "Only show automated builds")
	//	stars := cmd.Uint([]string{"s", "-stars"}, 0, "Only displays with at least x stars")
	filter := cmd.String([]string{"f", "-filter"}, "", "Use filters (is-automated, is-official, has-stars)")

	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	// Parse --filter options and return the map with these values:
	// * is-automated: Bool
	// * is-official: Bool
	// * has-stars: Number
	filterOptions := make(map[string]string)
	if *filter != "" {
		filterOptions = scanFilterOptions(*filter)
	}

	name := cmd.Arg(0)
	v := url.Values{}
	v.Set("term", name)

	indexInfo, err := registry.ParseSearchIndexInfo(name)
	if err != nil {
		return err
	}

	authConfig := cli.resolveAuthConfig(cli.configFile.AuthConfigs, indexInfo)
	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(indexInfo, "search")

	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImageSearchOptions{
		Term:         name,
		RegistryAuth: encodedAuth,
	}

	unorderedResults, err := cli.client.ImageSearch(options, requestPrivilege)
	if err != nil {
		return err
	}

	results := searchResultsByStars(unorderedResults)
	sort.Sort(results)

	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	for _, res := range results {
		//		if (*automated && !res.IsAutomated) || (int(*stars) > res.StarCount) {
		if filterOptions["is-automated"] == "true" && !res.IsAutomated {
			continue
		}
		if filterOptions["is-official"] == "true" && !res.IsOfficial {
			continue
		}
		has_stars, _ := strconv.Atoi(filterOptions["has-stars"])
		if has_stars > res.StarCount {
			continue
		}
		desc := strings.Replace(res.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = stringutils.Truncate(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", res.Name, desc, res.StarCount)
		if res.IsOfficial {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if res.IsAutomated || res.IsTrusted {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}

// SearchResultsByStars sorts search results in descending order by number of stars.
type searchResultsByStars []registrytypes.SearchResult

func (r searchResultsByStars) Len() int           { return len(r) }
func (r searchResultsByStars) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r searchResultsByStars) Less(i, j int) bool { return r[j].StarCount < r[i].StarCount }

func scanFilterOptions(filter string) map[string]string {
	options := map[string]string{
		"is-automated": "",
		"is-official":  "",
		"has-stars":    "",
	}

	optionsFields := strings.Fields(filter)

	for _, option := range optionsFields {
		if strings.Contains(option, "=") {
			val := strings.Split(option, "=")[1]
			if !(val != "true" && val != "false" && !unicode.IsNumber(rune(val[0]))) {
				key := strings.Split(option, "=")[0]
				options[key] = val
			}
		}
	}

	return options
}
