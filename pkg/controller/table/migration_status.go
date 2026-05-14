package table

import (
	"context"

	"github.com/pkg/errors"
	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileTable) upsertMigrationWithStatus(ctx context.Context, owner client.Object, migration *schemasv1alpha4.Migration) error {
	var existingMigration schemasv1alpha4.Migration
	err := r.Get(ctx, types.NamespacedName{
		Name:      migration.Name,
		Namespace: migration.Namespace,
	}, &existingMigration)

	if kuberneteserrors.IsNotFound(err) {
		createMigration := migration.DeepCopy()
		createMigration.Status = schemasv1alpha4.MigrationStatus{}

		if err := controllerutil.SetControllerReference(owner, createMigration, r.scheme); err != nil {
			return errors.Wrap(err, "failed to set owner on migration")
		}

		if err := r.Create(ctx, createMigration); err != nil {
			return errors.Wrap(err, "failed to create migration resource")
		}

		createMigration.Status = migration.Status
		if err := r.Status().Update(ctx, createMigration); err != nil {
			return errors.Wrap(err, "failed to update migration status")
		}

		return nil
	}

	if err != nil {
		return errors.Wrap(err, "failed to get existing migration")
	}

	existingMigration.Spec = migration.Spec
	if err := r.Update(ctx, &existingMigration); err != nil {
		return errors.Wrap(err, "failed to update migration resource")
	}

	existingMigration.Status = migration.Status
	if err := r.Status().Update(ctx, &existingMigration); err != nil {
		return errors.Wrap(err, "failed to update migration status")
	}

	return nil
}
