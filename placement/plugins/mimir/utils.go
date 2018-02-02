package mimir

import (
	"fmt"

	"code.uber.internal/infra/peloton/mimir-lib/model/labels"
	"code.uber.internal/infra/peloton/mimir-lib/model/metrics"
	"code.uber.internal/infra/peloton/mimir-lib/model/placement"
)

// Group is a helper to be able to make structured logging of the Mimir library types.
type Group struct {
	Name      string             `json:"name"`
	Labels    map[string]int     `json:"labels"`
	Metrics   map[string]float64 `json:"metrics"`
	Relations map[string]int     `json:"relations"`
	Entities  map[string]*Entity `json:"entities"`
}

// dumpGroup converts a Mimir group into a group who's structure we can log.
func dumpGroup(group *placement.Group) *Group {
	entities := map[string]*Entity{}
	for name, entity := range group.Entities {
		entities[name] = dumpEntity(entity)
	}
	return &Group{
		Name:      group.Name,
		Labels:    dumpLabelBag(group.Labels),
		Metrics:   dumpMetricSet(group.Metrics),
		Relations: dumpLabelBag(group.Relations),
		Entities:  entities,
	}
}

// Entity is a helper to be able to make structured logging of the Mimir library types.
type Entity struct {
	Name      string             `json:"name"`
	Relations map[string]int     `json:"relations"`
	Metrics   map[string]float64 `json:"metrics"`
}

// dumpEntity converts a Mimir entity into an entity who's structure we can log.
func dumpEntity(entity *placement.Entity) *Entity {
	return &Entity{
		Name:      entity.Name,
		Relations: dumpLabelBag(entity.Relations),
		Metrics:   dumpMetricSet(entity.Metrics),
	}
}

// dumpLabelBag converts a Mimir label bag into map of labels to counts who's structure we can log.
func dumpLabelBag(bag *labels.LabelBag) map[string]int {
	result := map[string]int{}
	for _, label := range bag.Labels() {
		result[label.String()] = bag.Count(label)
	}
	return result
}

// dumpMetricSet converts a Mimir metric set into map of metrics to values who's structure we can log.
func dumpMetricSet(set *metrics.MetricSet) map[string]float64 {
	result := map[string]float64{}
	for _, metricType := range set.Types(metrics.All()) {
		result[fmt.Sprintf("%v (%v)", metricType.Name, metricType.Unit)] = set.Get(metricType)
	}
	return result
}