package shrub

type Variant struct {
	BuildName        string                  `json:"name,omitempty" yaml:"name,omitempty"`
	BuildDisplayName string                  `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	BatchTimeSecs    int                     `json:"batchtime,omitempty" yaml:"batchtime,omitempty"`
	TaskSpecs        []TaskSpec              `json:"tasks,omitmepty" yaml:"tasks,omitempty"`
	DistroRunOn      []string                `json:"run_on,omitempty" yaml:"run_on,omitempty"`
	Expansions       map[string]interface{}  `json:"expansions,omitempty" yaml:"expansions,omitempty"`
	DisplayTaskSpecs []DisplayTaskDefinition `json:"display_tasks,omitempty" yaml:"display_tasks,omitempty"`
}

type DisplayTaskDefinition struct {
	Name       string   `json:"name" yaml:"name"`
	Components []string `json:"execution_tasks" yaml:"execution_tasks"`
}

type TaskSpec struct {
	Name     string   `json:"name" yaml:"name"`
	Stepback bool     `json:"stepback,omitempty" yaml:"stepback,omitempty"`
	Distro   []string `json:"distros,omitempty" yaml:"distro,omitempty"`
}

func (v *Variant) Name(id string) *Variant                         { v.BuildName = id; return v }
func (v *Variant) DisplayName(id string) *Variant                  { v.BuildDisplayName = id; return v }
func (v *Variant) RunOn(distro string) *Variant                    { v.DistroRunOn = []string{distro}; return v }
func (v *Variant) TaskSpec(spec TaskSpec) *Variant                 { v.TaskSpecs = append(v.TaskSpecs, spec); return v }
func (v *Variant) SetExpansions(m map[string]interface{}) *Variant { v.Expansions = m; return v }
func (v *Variant) Expansion(k string, val interface{}) *Variant {
	if v.Expansions == nil {
		v.Expansions = make(map[string]interface{})
	}

	v.Expansions[k] = val

	return v
}

func (v *Variant) AddTasks(name ...string) *Variant {
	for _, n := range name {
		if n == "" {
			continue
		}

		v.TaskSpecs = append(v.TaskSpecs, TaskSpec{
			Name: n,
		})
	}
	return v
}

func (v *Variant) DisplayTasks(def ...DisplayTaskDefinition) *Variant {
	v.DisplayTaskSpecs = append(v.DisplayTaskSpecs, def...)
	return v
}
