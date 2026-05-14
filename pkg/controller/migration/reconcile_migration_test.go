package migration

import (
	"context"
	"testing"
	"time"

	databasesv1alpha4 "github.com/schemahero/schemahero/pkg/apis/databases/v1alpha4"
	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	testclient "github.com/schemahero/schemahero/pkg/client/schemaheroclientset/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_getDatabaseFromMigration(t *testing.T) {
	db := &databasesv1alpha4.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testdb",
			Namespace: "namespace1",
		},
	}
	table1 := &schemasv1alpha4.Table{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "table1",
			Namespace: "namespace1",
		},
		Spec: schemasv1alpha4.TableSpec{
			Database: "testdb",
		},
	}
	view1 := &schemasv1alpha4.View{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "view1",
			Namespace: "namespace1",
		},
		Spec: schemasv1alpha4.ViewSpec{
			Database: "testdb",
		},
	}

	schemasClient = testclient.NewSimpleClientset(table1, view1).SchemasV1alpha4()
	databasesClient = testclient.NewSimpleClientset(db).DatabasesV1alpha4()

	tests := []struct {
		name      string
		migration *schemasv1alpha4.Migration
		want      *databasesv1alpha4.Database
	}{
		{
			name: "db from table",
			migration: &schemasv1alpha4.Migration{
				Spec: schemasv1alpha4.MigrationSpec{
					TableNamespace: "namespace1",
					TableName:      "table1",
				},
			},
			want: db,
		},
		{
			name: "db from view",
			migration: &schemasv1alpha4.Migration{
				Spec: schemasv1alpha4.MigrationSpec{
					TableNamespace: "namespace1",
					TableName:      "view1",
				},
			},
			want: db,
		},
		{
			name: "unknown db",
			migration: &schemasv1alpha4.Migration{
				Spec: schemasv1alpha4.MigrationSpec{
					TableNamespace: "namespace1",
					TableName:      "unknown",
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := getDatabaseFromMigration(ctx, tt.migration)
			if tt.want != nil {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func Test_shouldApplyMigration(t *testing.T) {
	tests := []struct {
		name      string
		migration *schemasv1alpha4.Migration
		want      bool
	}{
		{
			name: "approved not executedm, should apply",
			migration: &schemasv1alpha4.Migration{
				Status: schemasv1alpha4.MigrationStatus{
					ApprovedAt: time.Now().Unix(),
					ExecutedAt: 0,
				},
			},
			want: true,
		},
		{
			name: "approved and executed, should not aply",
			migration: &schemasv1alpha4.Migration{
				Status: schemasv1alpha4.MigrationStatus{
					ApprovedAt: time.Now().Unix(),
					ExecutedAt: time.Now().Unix(),
				},
			},
			want: false,
		},
		{
			name: "not approved, should not apply",
			migration: &schemasv1alpha4.Migration{
				Status: schemasv1alpha4.MigrationStatus{
					ApprovedAt: 0,
					ExecutedAt: 0,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldApplyMigration(tt.migration)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPersistExecutedStatus_UsesStatusSubresource(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := schemasv1alpha4.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("add schemas api to scheme: %v", err)
	}

	migration := &schemasv1alpha4.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "abc1234", Namespace: "default"},
		Status: schemasv1alpha4.MigrationStatus{
			ApprovedAt: time.Now().Unix(),
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&schemasv1alpha4.Migration{}).
		WithObjects(migration).
		Build()

	r := &ReconcileMigration{Client: client, scheme: scheme}

	if err := r.persistExecutedStatus(context.Background(), migration); err != nil {
		t.Fatalf("persist executed status: %v", err)
	}

	var updated schemasv1alpha4.Migration
	if err := client.Get(context.Background(), types.NamespacedName{Name: migration.Name, Namespace: migration.Namespace}, &updated); err != nil {
		t.Fatalf("get updated migration: %v", err)
	}

	if updated.Status.Phase != schemasv1alpha4.Executed {
		t.Fatalf("expected phase %q, got %q", schemasv1alpha4.Executed, updated.Status.Phase)
	}
	if updated.Status.ExecutedAt == 0 {
		t.Fatalf("expected executedAt to be set")
	}
}
