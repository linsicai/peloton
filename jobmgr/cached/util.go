package cached

import (
	"context"

	"code.uber.internal/infra/peloton/storage"

	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/peloton"
)

// AbortJobUpdate is a helper function to abort a given job update.
// It is primarily used to abort previous updates when a new update
// overwrites the previous one or aborting a given update.
func AbortJobUpdate(
	ctx context.Context,
	updateID *peloton.UpdateID,
	updateStore storage.UpdateStore,
	updateFactory UpdateFactory) error {
	// ensure that the previous update is not already terminated
	updateInfo, err := updateStore.GetUpdateProgress(ctx, updateID)
	if err != nil {
		return err
	}

	if IsUpdateStateTerminal(updateInfo.GetState()) {
		return nil
	}

	// abort the previous non-terminal update
	cachedUpdate := updateFactory.GetUpdate(updateID)

	if cachedUpdate == nil {
		cachedUpdate = updateFactory.AddUpdate(updateID)
		if err = cachedUpdate.Recover(ctx); err != nil {
			// failed to recover previous update, fail this create request
			return err
		}
	}

	if err = cachedUpdate.Cancel(ctx); err != nil {
		// failed to cancel the previous update, since cannot run two
		// updates on the same job, fail this create request
		return err
	}

	return nil
}
