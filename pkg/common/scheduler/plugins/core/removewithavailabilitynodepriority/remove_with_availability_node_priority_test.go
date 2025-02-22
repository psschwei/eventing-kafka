/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package removewithavailabilitynodepriority

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing-kafka/pkg/common/scheduler"
	state "knative.dev/eventing-kafka/pkg/common/scheduler/state"
	tscheduler "knative.dev/eventing-kafka/pkg/common/scheduler/testing"
	listers "knative.dev/eventing/pkg/reconciler/testing/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

const (
	testNs        = "test-ns"
	sfsName       = "statefulset-name"
	vpodName      = "source-name"
	vpodNamespace = "source-namespace"
	numZones      = 3
	numNodes      = 6
)

func TestScore(t *testing.T) {
	testCases := []struct {
		name     string
		state    *state.State
		vpod     types.NamespacedName
		replicas int32
		podID    int32
		expected *state.Status
		expScore uint64
		args     interface{}
	}{
		{
			name: "no vpods, no pods",
			vpod: types.NamespacedName{},
			state: &state.State{StatefulSetName: sfsName, Replicas: 0,
				NodeSpread: map[types.NamespacedName]map[string]int32{}},
			replicas: 0,
			podID:    0,
			expected: state.NewStatus(state.Success),
			expScore: 0,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "no vpods, no pods, bad arg",
			vpod: types.NamespacedName{},
			state: &state.State{StatefulSetName: sfsName, Replicas: 0,
				NodeSpread: map[types.NamespacedName]map[string]int32{}},
			replicas: 0,
			podID:    0,
			expected: state.NewStatus(state.Unschedulable, ErrReasonInvalidArg),
			expScore: 0,
			args:     "{\"MaxSkewness\": 2}",
		},
		{
			name: "no vpods, no pods, no resource",
			vpod: types.NamespacedName{},
			state: &state.State{StatefulSetName: sfsName, Replicas: 1,
				NodeSpread: map[types.NamespacedName]map[string]int32{}},
			replicas: 0,
			podID:    1,
			expected: state.NewStatus(state.Error, ErrReasonNoResource),
			expScore: 0,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, one node, same pod filter",
			vpod: types.NamespacedName{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"},
			state: &state.State{StatefulSetName: sfsName, Replicas: 1,
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"}: {
						"node0": 5,
					},
				},
			},
			replicas: 1,
			podID:    0,
			expected: state.NewStatus(state.Success),
			expScore: math.MaxUint64,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "two vpods, one node, same pod filter",
			vpod: types.NamespacedName{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"},
			state: &state.State{StatefulSetName: sfsName, Replicas: 1,
				NodeSpread: map[types.NamespacedName]map[string]int32{
					{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"}: {
						"node0": 5,
					},
					{Name: vpodName + "-1", Namespace: vpodNamespace + "-1"}: {
						"node1": 4,
					},
				},
			},
			replicas: 1,
			podID:    0,
			expected: state.NewStatus(state.Success),
			expScore: math.MaxUint64,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, two nodes, same pod filter",
			vpod: types.NamespacedName{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"},
			state: &state.State{StatefulSetName: sfsName, Replicas: 2, NodeSpread: map[types.NamespacedName]map[string]int32{
				{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"}: {
					"node0": 5,
					"node1": 5,
					"node2": 3,
				},
			}},
			replicas: 2,
			podID:    1,
			expected: state.NewStatus(state.Success),
			expScore: math.MaxUint64 - 2,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, three nodes, same pod filter",
			vpod: types.NamespacedName{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"},
			state: &state.State{StatefulSetName: sfsName, Replicas: 3, NodeSpread: map[types.NamespacedName]map[string]int32{
				{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"}: {
					"node0": 5,
					"node1": 4,
					"node2": 3,
				},
			}},
			replicas: 3,
			podID:    1,
			expected: state.NewStatus(state.Success),
			expScore: math.MaxUint64 - 2,
			args:     "{\"MaxSkew\": 2}",
		},
		{
			name: "one vpod, five pods, same pod filter",
			vpod: types.NamespacedName{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"},
			state: &state.State{StatefulSetName: sfsName, Replicas: 5, NodeSpread: map[types.NamespacedName]map[string]int32{
				{Name: vpodName + "-0", Namespace: vpodNamespace + "-0"}: {
					"node0": 8,
					"node1": 4,
					"node2": 3,
				},
			}},
			replicas: 5,
			podID:    0,
			expected: state.NewStatus(state.Success),
			expScore: math.MaxUint64 - 7,
			args:     "{\"MaxSkew\": 2}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := tscheduler.SetupFakeContext(t)
			var plugin = &RemoveWithAvailabilityNodePriority{}

			name := plugin.Name()
			assert.Equal(t, name, state.RemoveWithAvailabilityNodePriority)

			nodelist := make([]*v1.Node, 0)
			podlist := make([]runtime.Object, 0)

			for i := int32(0); i < numZones; i++ {
				for j := int32(0); j < numNodes/numZones; j++ {
					nodeName := "node" + fmt.Sprint((j*((numNodes/numZones)+1))+i)
					zoneName := "zone" + fmt.Sprint(i)
					node, err := kubeclient.Get(ctx).CoreV1().Nodes().Create(ctx, tscheduler.MakeNode(nodeName, zoneName), metav1.CreateOptions{})
					if err != nil {
						t.Fatal("unexpected error", err)
					}
					nodelist = append(nodelist, node)
				}
			}

			for i := int32(0); i < tc.replicas; i++ {
				nodeName := "node" + fmt.Sprint(i)
				podName := sfsName + "-" + fmt.Sprint(i)
				pod, err := kubeclient.Get(ctx).CoreV1().Pods(testNs).Create(ctx, tscheduler.MakePod(testNs, podName, nodeName), metav1.CreateOptions{})
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				podlist = append(podlist, pod)
			}

			nodeToZoneMap := make(map[string]string)
			for i := 0; i < len(nodelist); i++ {
				node := nodelist[i]
				zoneName, ok := node.GetLabels()[scheduler.ZoneLabel]
				if !ok {
					continue //ignore node that doesn't have zone info (maybe a test setup or control node)
				}
				nodeToZoneMap[node.Name] = zoneName
			}

			lsp := listers.NewListers(podlist)
			tc.state.PodLister = lsp.GetPodLister().Pods(testNs)
			tc.state.NodeToZoneMap = nodeToZoneMap

			score, status := plugin.Score(ctx, tc.args, tc.state, tc.state.SchedulablePods, tc.vpod, tc.podID)
			if score != tc.expScore {
				t.Errorf("unexpected score, got %v, want %v", score, tc.expScore)
			}
			if !reflect.DeepEqual(status, tc.expected) {
				t.Errorf("unexpected status, got %v, want %v", status, tc.expected)
			}
		})
	}
}
