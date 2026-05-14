package table

import (
	"context"
	"testing"

	databasesv1alpha4 "github.com/schemahero/schemahero/pkg/apis/databases/v1alpha4"
	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpsertMigrationWithStatus_CreateSetsStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := schemasv1alpha4.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("add schemas api to scheme: %v", err)
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&schemasv1alpha4.Migration{}).
		Build()

	r := &ReconcileTable{Client: client, scheme: scheme}
	owner := &schemasv1alpha4.Table{ObjectMeta: metav1.ObjectMeta{Name: "users", Namespace: "default"}}
	migration := &schemasv1alpha4.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "abc1234", Namespace: "default"},
		Spec: schemasv1alpha4.MigrationSpec{
			DatabaseName:   "db",
			TableName:      "users",
			TableNamespace: "default",
			GeneratedDDL:   "create table users ();",
		},
		Status: schemasv1alpha4.MigrationStatus{Phase: schemasv1alpha4.Planned},
	}

	if err := r.upsertMigrationWithStatus(context.Background(), owner, migration); err != nil {
		t.Fatalf("upsert migration: %v", err)
	}

	var created schemasv1alpha4.Migration
	if err := client.Get(context.Background(), types.NamespacedName{Name: "abc1234", Namespace: "default"}, &created); err != nil {
		t.Fatalf("get created migration: %v", err)
	}

	if created.Status.Phase != schemasv1alpha4.Planned {
		t.Fatalf("expected phase %q, got %q", schemasv1alpha4.Planned, created.Status.Phase)
	}
}

func TestUpsertMigrationWithStatus_UpdateSetsStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := schemasv1alpha4.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("add schemas api to scheme: %v", err)
	}
	if err := databasesv1alpha4.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("add databases api to scheme: %v", err)
	}

	existing := &schemasv1alpha4.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "abc1234", Namespace: "default"},
		Spec: schemasv1alpha4.MigrationSpec{
			DatabaseName:   "db",
			TableName:      "users",
			TableNamespace: "default",
			GeneratedDDL:   "old ddl",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&schemasv1alpha4.Migration{}).
		WithObjects(existing).
		Build()

	r := &ReconcileTable{Client: client, scheme: scheme}
	owner := &databasesv1alpha4.Database{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"}}
	migration := &schemasv1alpha4.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "abc1234", Namespace: "default"},
		Spec: schemasv1alpha4.MigrationSpec{
			DatabaseName:   "db",
			TableName:      "users",
			TableNamespace: "default",
			GeneratedDDL:   "new ddl",
		},
		Status: schemasv1alpha4.MigrationStatus{Phase: schemasv1alpha4.Planned},
	}

	if err := r.upsertMigrationWithStatus(context.Background(), owner, migration); err != nil {
		t.Fatalf("upsert migration: %v", err)
	}

	var updated schemasv1alpha4.Migration
	if err := client.Get(context.Background(), types.NamespacedName{Name: "abc1234", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get updated migration: %v", err)
	}

	if updated.Status.Phase != schemasv1alpha4.Planned {
		t.Fatalf("expected phase %q, got %q", schemasv1alpha4.Planned, updated.Status.Phase)
	}
	if updated.Spec.GeneratedDDL != "new ddl" {
		t.Fatalf("expected updated ddl, got %q", updated.Spec.GeneratedDDL)
	}
}
