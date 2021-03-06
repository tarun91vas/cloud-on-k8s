// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package telemetry

import (
	"context"
	"testing"

	"github.com/elastic/cloud-on-k8s/pkg/about"
	apmv1 "github.com/elastic/cloud-on-k8s/pkg/apis/apm/v1"
	beatv1beta1 "github.com/elastic/cloud-on-k8s/pkg/apis/beat/v1beta1"
	commonv1 "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	entv1beta1 "github.com/elastic/cloud-on-k8s/pkg/apis/enterprisesearch/v1beta1"
	kbv1 "github.com/elastic/cloud-on-k8s/pkg/apis/kibana/v1"
	"github.com/elastic/cloud-on-k8s/pkg/controller/kibana"
	"github.com/elastic/cloud-on-k8s/pkg/utils/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testOperatorInfo = about.OperatorInfo{
	OperatorUUID:            "15039433-f873-41bd-b6e7-10ee3665cafa",
	CustomOperatorNamespace: true,
	Distribution:            "v1.16.13-gke.1",
	BuildInfo: about.BuildInfo{
		Version:  "1.1.0",
		Hash:     "b5316231",
		Date:     "2019-09-20T07:00:00Z",
		Snapshot: "true",
	},
}

func TestMarshalTelemetry(t *testing.T) {
	for _, tt := range []struct {
		name  string
		info  about.OperatorInfo
		stats map[string]interface{}
		want  string
	}{
		{
			name:  "empty",
			info:  about.OperatorInfo{},
			stats: nil,
			want: `eck:
  build:
    date: ""
    hash: ""
    snapshot: ""
    version: ""
  custom_operator_namespace: false
  distribution: ""
  operator_uuid: ""
  stats: null
`,
		},
		{
			name: "not empty",
			info: testOperatorInfo,
			stats: map[string]interface{}{
				"apms": map[string]interface{}{
					"pod_count":      2,
					"resource_count": 1,
				},
			},
			want: `eck:
  build:
    date: "2019-09-20T07:00:00Z"
    hash: b5316231
    snapshot: "true"
    version: 1.1.0
  custom_operator_namespace: true
  distribution: v1.16.13-gke.1
  operator_uuid: 15039433-f873-41bd-b6e7-10ee3665cafa
  stats:
    apms:
      pod_count: 2
      resource_count: 1
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, gotErr := marshalTelemetry(tt.info, tt.stats)
			require.NoError(t, gotErr)
			require.Equal(t, tt.want, string(gotBytes))
		})
	}
}

func TestNewReporter(t *testing.T) {
	createKbAndSecret := func(name, namespace string, count int32) (kbv1.Kibana, corev1.Secret) {
		kb := kbv1.Kibana{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: kbv1.KibanaSpec{
				Count: count,
			},
		}
		return kb, corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kibana.SecretName(kb),
				Namespace: namespace,
			},
		}
	}

	kb1, s1 := createKbAndSecret("kb1", "ns1", 1)
	kb2, s2 := createKbAndSecret("kb2", "ns2", 2)
	kb3, s3 := createKbAndSecret("kb3", "ns3", 3)

	client := k8s.FakeClient(
		&kb1,
		&kb2,
		&kb3,
		&s1,
		&s2,
		&s3,
		&esv1.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns1",
			},
			Status: esv1.ElasticsearchStatus{
				AvailableNodes: 3,
			},
		},
		&apmv1.ApmServer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns1",
			},
			Status: apmv1.ApmServerStatus{
				DeploymentStatus: commonv1.DeploymentStatus{
					AvailableNodes: 2,
				},
			},
		},
		&entv1beta1.EnterpriseSearch{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns1",
			},
			Status: entv1beta1.EnterpriseSearchStatus{
				DeploymentStatus: commonv1.DeploymentStatus{
					AvailableNodes: 3,
				},
			},
		},
		&beatv1beta1.Beat{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "beat1",
				Namespace: "ns1",
			},
			Spec: beatv1beta1.BeatSpec{
				Type: "filebeat",
			},
			Status: beatv1beta1.BeatStatus{
				AvailableNodes: 7,
			},
		},
		&beatv1beta1.Beat{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "beat2",
				Namespace: "ns2",
			},
			Spec: beatv1beta1.BeatSpec{
				Type: "metricbeat",
			},
			Status: beatv1beta1.BeatStatus{
				AvailableNodes: 1,
			},
		},
		&beatv1beta1.Beat{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "beat3",
				Namespace: "ns3",
			},
			Spec: beatv1beta1.BeatSpec{
				Type: "metricbeat",
			},
			Status: beatv1beta1.BeatStatus{
				AvailableNodes: 7,
			},
		},
	)

	r := NewReporter(testOperatorInfo, client, []string{kb1.Namespace, kb2.Namespace})
	r.report()

	wantData := map[string][]byte{
		"telemetry.yml": []byte(`eck:
  build:
    date: "2019-09-20T07:00:00Z"
    hash: b5316231
    snapshot: "true"
    version: 1.1.0
  custom_operator_namespace: true
  distribution: v1.16.13-gke.1
  operator_uuid: 15039433-f873-41bd-b6e7-10ee3665cafa
  stats:
    apms:
      pod_count: 2
      resource_count: 1
    beats:
      auditbeat_count: 0
      filebeat_count: 1
      heartbeat_count: 0
      journalbeat_count: 0
      metricbeat_count: 1
      packetbeat_count: 0
      pod_count: 8
      resource_count: 2
    elasticsearches:
      pod_count: 3
      resource_count: 1
    enterprisesearches:
      pod_count: 3
      resource_count: 1
    kibanas:
      pod_count: 0
      resource_count: 2
`),
	}

	require.NoError(t, client.Get(context.Background(), k8s.ExtractNamespacedName(&s1), &s1))
	require.Equal(t, wantData, s1.Data)

	require.NoError(t, client.Get(context.Background(), k8s.ExtractNamespacedName(&s2), &s2))
	require.Equal(t, wantData, s2.Data)

	require.NoError(t, client.Get(context.Background(), k8s.ExtractNamespacedName(&s3), &s3))
	require.Nil(t, s3.Data)
}
