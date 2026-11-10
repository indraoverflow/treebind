/*
Copyright The Treebind Authors.

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

package controller

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	treebindv1alpha1 "github.com/mohanedmoh/treebind/api/v1alpha1"
)

// newTreebind returns the smallest Treebind the CRD schema accepts, so that a
// test which cares about one field starts from a valid object.
func newTreebind(name string) *treebindv1alpha1.Treebind {
	return &treebindv1alpha1.Treebind{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: treebindv1alpha1.TreebindSpec{
			Repository: treebindv1alpha1.RepositorySpec{
				URL:    "ssh://git@github.com/example/dashboards.git",
				Branch: "main",
			},
			Target: treebindv1alpha1.TargetSpec{ClaimName: "grafana-data"},
			Sync:   treebindv1alpha1.SyncSpec{Direction: treebindv1alpha1.SyncDirectionBidirectional},
		},
	}
}

// These tests run against the CRD in config/crd/bases, which envtest installs,
// so they check the generated schema and not only the Go types.
var _ = Describe("Treebind schema", func() {
	ctx := context.Background()

	It("accepts every field of the sample manifest and returns it unchanged", func() {
		tb := newTreebind("schema-valid")
		tb.Spec.Repository.Path = "dashboards"
		tb.Spec.Repository.Auth = &treebindv1alpha1.AuthSpec{
			SSHKeySecretRef:        &treebindv1alpha1.KeySelector{Name: "dashboards-deploy-key", Key: "id_ed25519"},
			KnownHostsConfigMapRef: &treebindv1alpha1.KeySelector{Name: "git-known-hosts", Key: "known_hosts"},
		}
		tb.Spec.Target.SubPath = "dashboards"
		tb.Spec.Target.MountPath = "/work"
		tb.Spec.Sync.Interval = &metav1.Duration{Duration: 30 * time.Second}
		tb.Spec.Sync.SettleTime = &metav1.Duration{Duration: 5 * time.Second}
		tb.Spec.Sync.ConflictPolicy = treebindv1alpha1.ConflictPolicyFollowRemote
		tb.Spec.Commit = &treebindv1alpha1.CommitSpec{
			AuthorName:      "treebind",
			AuthorEmail:     "treebind@example.org",
			MessageTemplate: "treebind: sync {{ .Count }} file(s)",
		}
		tb.Spec.Paths = &treebindv1alpha1.PathsSpec{Exclude: []string{"*.tmp", "*.swp"}}

		Expect(k8sClient.Create(ctx, tb)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, tb)).To(Succeed()) })

		got := &treebindv1alpha1.Treebind{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(tb), got)).To(Succeed())
		Expect(got.Spec).To(Equal(tb.Spec))
	})

	It("accepts the sample manifest", func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "samples", "treebind_v1alpha1_treebind.yaml"))
		Expect(err).NotTo(HaveOccurred())

		tb := &treebindv1alpha1.Treebind{}
		Expect(yaml.UnmarshalStrict(data, tb)).To(Succeed())
		tb.Namespace = "default"

		Expect(k8sClient.Create(ctx, tb)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, tb)).To(Succeed()) })
	})

	It("stores the status fields through the status subresource", func() {
		tb := newTreebind("schema-status")
		Expect(k8sClient.Create(ctx, tb)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, tb)).To(Succeed()) })

		syncTime := metav1.NewTime(time.Now().Truncate(time.Second))
		tb.Status = treebindv1alpha1.TreebindStatus{
			ObservedGeneration: tb.Generation,
			LastSyncedCommit:   "9f2c1ab0d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9",
			LastSyncTime:       &syncTime,
		}
		Expect(k8sClient.Status().Update(ctx, tb)).To(Succeed())

		got := &treebindv1alpha1.Treebind{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(tb), got)).To(Succeed())
		Expect(got.Status.ObservedGeneration).To(Equal(tb.Generation))
		Expect(got.Status.LastSyncedCommit).To(Equal("9f2c1ab0d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"))
		Expect(got.Status.LastSyncTime).NotTo(BeNil())
		Expect(got.Status.LastSyncTime.Time).To(BeTemporally("==", syncTime.Time))
	})

	DescribeTable("rejects an object that breaks the schema",
		func(mutate func(*treebindv1alpha1.Treebind), field string) {
			tb := newTreebind("schema-invalid")
			mutate(tb)

			err := k8sClient.Create(ctx, tb)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got: %v", err)
			Expect(err).To(MatchError(ContainSubstring(field)))
		},
		Entry("without a repository URL",
			func(tb *treebindv1alpha1.Treebind) { tb.Spec.Repository.URL = "" },
			"spec.repository.url"),
		Entry("without a claim name",
			func(tb *treebindv1alpha1.Treebind) { tb.Spec.Target.ClaimName = "" },
			"spec.target.claimName"),
		Entry("with a direction that is neither pull nor bidirectional",
			func(tb *treebindv1alpha1.Treebind) { tb.Spec.Sync.Direction = "sideways" },
			"spec.sync.direction"),
		Entry("with an unknown conflict policy",
			func(tb *treebindv1alpha1.Treebind) { tb.Spec.Sync.ConflictPolicy = "merge" },
			"spec.sync.conflictPolicy"),
		Entry("with a key reference that names no key",
			func(tb *treebindv1alpha1.Treebind) {
				tb.Spec.Repository.Auth = &treebindv1alpha1.AuthSpec{
					SSHKeySecretRef: &treebindv1alpha1.KeySelector{Name: "dashboards-deploy-key"},
				}
			},
			"spec.repository.auth.sshKeySecretRef.key"),
	)
})
