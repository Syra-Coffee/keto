package check

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ory/keto/internal/check/checkgroup"
	"github.com/ory/keto/internal/expand"
)

type binaryOperator = func(ctx context.Context, checks []checkgroup.CheckFunc) checkgroup.Result

func or(ctx context.Context, checks []checkgroup.CheckFunc) checkgroup.Result {
	if len(checks) == 0 {
		return checkgroup.ResultNotMember
	}

	resultCh := make(chan checkgroup.Result, len(checks))
	childCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	for _, check := range checks {
		go check(childCtx, resultCh)
	}

	for i := 0; i < len(checks); i++ {
		select {
		case result := <-resultCh:
			// We return either the first error or the first success.
			if result.Err != nil || result.Membership == checkgroup.IsMember {
				return result
			}
		case <-ctx.Done():
			return checkgroup.Result{Err: errors.WithStack(ctx.Err())}
		}
	}

	return checkgroup.ResultNotMember
}

func and(ctx context.Context, checks []checkgroup.CheckFunc) checkgroup.Result {
	if len(checks) == 0 {
		return checkgroup.ResultNotMember
	}

	resultCh := make(chan checkgroup.Result, len(checks))
	childCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	for _, check := range checks {
		go check(childCtx, resultCh)
	}

	tree := &expand.Tree{
		Type:     expand.Intersection,
		Children: []*expand.Tree{},
	}

	for i := 0; i < len(checks); i++ {
		select {
		case result := <-resultCh:
			// We return fast on either an error or if a subcheck returns "not a
			// member".
			if result.Err != nil || result.Membership != checkgroup.IsMember {
				return checkgroup.Result{Err: result.Err, Membership: checkgroup.NotMember}
			} else {
				tree.Children = append(tree.Children, result.Tree)
			}
		case <-ctx.Done():
			return checkgroup.Result{Err: errors.WithStack(ctx.Err())}
		}
	}

	return checkgroup.Result{
		Membership: checkgroup.IsMember,
		Tree:       tree,
	}
}