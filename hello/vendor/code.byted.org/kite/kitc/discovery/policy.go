package discovery

type FilterFunc func(expect, actual string) bool

/*   DiscoveryFilterPolicy is design to support customizable filter policy on cluster or env tags
 * during service discovery process.
 *   Instances from discovery component will be filtered by `FilterRule` first. If the result is
 * not empty, is will be passed to later process. Otherwise, the original instances will be filtered
 * by `BackupRule` instead to retrieve different result.
 */
type DiscoveryFilterPolicy struct {
	FilterRule FilterFunc
	BackupRule FilterFunc
}

type DiscoveryPolicy struct {
	ClusterPolicy *DiscoveryFilterPolicy
	EnvPolicy     *DiscoveryFilterPolicy
}

// Provided to simplify user customization
var (
	DefaultClusterFilterRule FilterFunc = ExactlyMatch
	DefaultClusterBackupRule FilterFunc = nil

	DefaultEnvFilterRule FilterFunc = func(expect, actual string) bool {
		if expect == actual {
			return true
		}
		return expect == "prod" && actual == "canary"
	}
	DefaultEnvBackupRule FilterFunc = func(expect, actual string) bool {
		return actual == "prod" || actual == "canary"
	}

	DefaultClusterPolicy = &DiscoveryFilterPolicy{
		FilterRule: DefaultClusterFilterRule,
		BackupRule: DefaultClusterBackupRule,
	}
	DefaultEnvPolicy = &DiscoveryFilterPolicy{
		FilterRule: DefaultEnvFilterRule,
		BackupRule: DefaultEnvBackupRule,
	}
)

func ExactlyMatch(expect, actual string) bool {
	return expect == actual
}

func AcceptAll(expect, actual string) bool {
	return true
}

func NewDiscoveryPolicy() *DiscoveryPolicy {
	policy := &DiscoveryPolicy{
		DefaultClusterPolicy,
		DefaultEnvPolicy,
	}
	return policy
}

func (dfp *DiscoveryFilterPolicy) AllFilters() []FilterFunc {
	res := make([]FilterFunc, 0)
	if dfp.FilterRule != nil {
		res = append(res, dfp.FilterRule)
	}
	if dfp.BackupRule != nil {
		res = append(res, dfp.BackupRule)
	}
	if len(res) == 0 {
		res = append(res, AcceptAll)
	}
	return res
}

func (dp *DiscoveryPolicy) Filter(ins []*Instance, cluster, env string) []*Instance {

	// Unify "" and "default" to make filter process simple
	if cluster == "" {
		cluster = "default"
	}
	// Unify "" and "prod" to make filter process simple
	if env == "" {
		env = "prod"
	}

	remains := make([]*Instance, 0, len(ins))
	for _, clusterFilter := range dp.ClusterPolicy.AllFilters() {
		for _, envFilter := range dp.EnvPolicy.AllFilters() {
			for _, in := range ins {
				if clusterFilter(cluster, in.Cluster()) && envFilter(env, in.Env()) {
					remains = append(remains, in)
				}
			}
			if len(remains) > 0 {
				goto DONE
			}
		}
	}
DONE:
	return remains
}
