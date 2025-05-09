package runtime_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	version "github.com/hashicorp/go-version"
	. "github.com/petergtz/pegomock/v4"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/runtime"
	runtimemocks "github.com/runatlantis/atlantis/server/core/runtime/mocks"
	runtimemodels "github.com/runatlantis/atlantis/server/core/runtime/models"
	tf "github.com/runatlantis/atlantis/server/core/terraform"
	"github.com/runatlantis/atlantis/server/core/terraform/mocks"
	tfclientmocks "github.com/runatlantis/atlantis/server/core/terraform/tfclient/mocks"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"

	. "github.com/runatlantis/atlantis/testing"
)

func TestRun_NoDir(t *testing.T) {
	o := runtime.ApplyStepRunner{
		TerraformExecutor: nil,
	}
	_, err := o.Run(command.ProjectContext{
		RepoRelDir: ".",
		Workspace:  "workspace",
	}, nil, "/nonexistent/path", map[string]string(nil))
	ErrEquals(t, "no plan found at path \".\" and workspace \"workspace\"–did you run plan?", err)
}

func TestRun_NoPlanFile(t *testing.T) {
	tmpDir := t.TempDir()
	o := runtime.ApplyStepRunner{
		TerraformExecutor: nil,
	}
	_, err := o.Run(command.ProjectContext{
		RepoRelDir: ".",
		Workspace:  "workspace",
	}, nil, tmpDir, map[string]string(nil))
	ErrEquals(t, "no plan found at path \".\" and workspace \"workspace\"–did you run plan?", err)
}

func TestRun_Success(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "workspace.tfplan")
	err := os.WriteFile(planPath, nil, 0600)
	logger := logging.NewNoopLogger(t)
	ctx := command.ProjectContext{
		Log:                logger,
		Workspace:          "workspace",
		RepoRelDir:         ".",
		EscapedCommentArgs: []string{"comment", "args"},
	}
	Ok(t, err)

	RegisterMockTestingT(t)
	terraform := tfclientmocks.NewMockClient()
	mockDownloader := mocks.NewMockDownloader()
	tfDistribution := tf.NewDistributionTerraformWithDownloader(mockDownloader)
	o := runtime.ApplyStepRunner{
		TerraformExecutor:     terraform,
		DefaultTFDistribution: tfDistribution,
	}

	When(terraform.RunCommandWithVersion(Any[command.ProjectContext](), Any[string](), Any[[]string](), Any[map[string]string](), Any[tf.Distribution](), Any[*version.Version](), Any[string]())).
		ThenReturn("output", nil)
	output, err := o.Run(ctx, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	Ok(t, err)
	Equals(t, "output", output)
	terraform.VerifyWasCalledOnce().RunCommandWithVersion(ctx, tmpDir, []string{"apply", "-input=false", "extra", "args", "comment", "args", fmt.Sprintf("%q", planPath)}, map[string]string(nil), tfDistribution, nil, "workspace")
	_, err = os.Stat(planPath)
	Assert(t, os.IsNotExist(err), "planfile should be deleted")
}

func TestRun_AppliesCorrectProjectPlan(t *testing.T) {
	// When running for a project, the planfile has a different name.
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "projectname-default.tfplan")
	err := os.WriteFile(planPath, nil, 0600)

	logger := logging.NewNoopLogger(t)
	ctx := command.ProjectContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		ProjectName:        "projectname",
		EscapedCommentArgs: []string{"comment", "args"},
	}
	Ok(t, err)

	RegisterMockTestingT(t)
	terraform := tfclientmocks.NewMockClient()
	mockDownloader := mocks.NewMockDownloader()
	tfDistribution := tf.NewDistributionTerraformWithDownloader(mockDownloader)
	o := runtime.ApplyStepRunner{
		TerraformExecutor:     terraform,
		DefaultTFDistribution: tfDistribution,
	}
	When(terraform.RunCommandWithVersion(Any[command.ProjectContext](), Any[string](), Any[[]string](), Any[map[string]string](), Any[tf.Distribution](), Any[*version.Version](), Any[string]())).
		ThenReturn("output", nil)
	output, err := o.Run(ctx, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	Ok(t, err)
	Equals(t, "output", output)
	terraform.VerifyWasCalledOnce().RunCommandWithVersion(ctx, tmpDir, []string{"apply", "-input=false", "extra", "args", "comment", "args", fmt.Sprintf("%q", planPath)}, map[string]string(nil), tfDistribution, nil, "default")
	_, err = os.Stat(planPath)
	Assert(t, os.IsNotExist(err), "planfile should be deleted")
}

func TestApplyStepRunner_TestRun_UsesConfiguredTFVersion(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "workspace.tfplan")
	err := os.WriteFile(planPath, nil, 0600)
	Ok(t, err)

	logger := logging.NewNoopLogger(t)
	tfVersion, _ := version.NewVersion("0.11.0")
	ctx := command.ProjectContext{
		Workspace:          "workspace",
		RepoRelDir:         ".",
		EscapedCommentArgs: []string{"comment", "args"},
		TerraformVersion:   tfVersion,
		Log:                logger,
	}

	RegisterMockTestingT(t)
	terraform := tfclientmocks.NewMockClient()
	mockDownloader := mocks.NewMockDownloader()
	tfDistribution := tf.NewDistributionTerraformWithDownloader(mockDownloader)
	o := runtime.ApplyStepRunner{
		TerraformExecutor:     terraform,
		DefaultTFDistribution: tfDistribution,
	}
	When(terraform.RunCommandWithVersion(Any[command.ProjectContext](), Any[string](), Any[[]string](), Any[map[string]string](), Any[tf.Distribution](), Any[*version.Version](), Any[string]())).
		ThenReturn("output", nil)
	output, err := o.Run(ctx, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	Ok(t, err)
	Equals(t, "output", output)
	terraform.VerifyWasCalledOnce().RunCommandWithVersion(ctx, tmpDir, []string{"apply", "-input=false", "extra", "args", "comment", "args", fmt.Sprintf("%q", planPath)}, map[string]string(nil), tfDistribution, tfVersion, "workspace")
	_, err = os.Stat(planPath)
	Assert(t, os.IsNotExist(err), "planfile should be deleted")
}

func TestApplyStepRunner_TestRun_UsesConfiguredDistribution(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "workspace.tfplan")
	err := os.WriteFile(planPath, nil, 0600)
	Ok(t, err)

	logger := logging.NewNoopLogger(t)
	mockDownloader := mocks.NewMockDownloader()
	tfDistribution := tf.NewDistributionTerraformWithDownloader(mockDownloader)
	tfVersion, _ := version.NewVersion("0.11.0")
	projTFDistribution := "opentofu"
	ctx := command.ProjectContext{
		Workspace:             "workspace",
		RepoRelDir:            ".",
		EscapedCommentArgs:    []string{"comment", "args"},
		TerraformDistribution: &projTFDistribution,
		Log:                   logger,
	}

	RegisterMockTestingT(t)
	terraform := tfclientmocks.NewMockClient()
	o := runtime.ApplyStepRunner{
		TerraformExecutor:     terraform,
		DefaultTFDistribution: tfDistribution,
		DefaultTFVersion:      tfVersion,
	}
	When(terraform.RunCommandWithVersion(Any[command.ProjectContext](), Any[string](), Any[[]string](), Any[map[string]string](), NotEq[tf.Distribution](tfDistribution), Any[*version.Version](), Any[string]())).
		ThenReturn("output", nil)
	output, err := o.Run(ctx, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	Ok(t, err)
	Equals(t, "output", output)
	terraform.VerifyWasCalledOnce().RunCommandWithVersion(Eq(ctx), Eq(tmpDir), Eq([]string{"apply", "-input=false", "extra", "args", "comment", "args", fmt.Sprintf("%q", planPath)}), Eq(map[string]string(nil)), NotEq[tf.Distribution](tfDistribution), Eq(tfVersion), Eq("workspace"))
	_, err = os.Stat(planPath)
	Assert(t, os.IsNotExist(err), "planfile should be deleted")
}

// Apply ignores the -target flag when used with a planfile so we should give
// an error if it's being used with -target.
func TestRun_UsingTarget(t *testing.T) {
	logger := logging.NewNoopLogger(t)
	cases := []struct {
		commentFlags []string
		extraArgs    []string
		expErr       bool
	}{
		{
			commentFlags: []string{"-target", "mytarget"},
			expErr:       true,
		},
		{
			commentFlags: []string{"-target=mytarget"},
			expErr:       true,
		},
		{
			extraArgs: []string{"-target", "mytarget"},
			expErr:    true,
		},
		{
			extraArgs: []string{"-target=mytarget"},
			expErr:    true,
		},
		{
			commentFlags: []string{"-target", "mytarget"},
			extraArgs:    []string{"-target=mytarget"},
			expErr:       true,
		},
		// Test false positives.
		{
			commentFlags: []string{"-targethahagotcha"},
			expErr:       false,
		},
		{
			extraArgs: []string{"-targethahagotcha"},
			expErr:    false,
		},
		{
			commentFlags: []string{"-targeted=weird"},
			expErr:       false,
		},
		{
			extraArgs: []string{"-targeted=weird"},
			expErr:    false,
		},
	}

	RegisterMockTestingT(t)

	for _, c := range cases {
		descrip := fmt.Sprintf("comments flags: %s extra args: %s",
			strings.Join(c.commentFlags, ", "), strings.Join(c.extraArgs, ", "))
		t.Run(descrip, func(t *testing.T) {
			tmpDir := t.TempDir()
			planPath := filepath.Join(tmpDir, "workspace.tfplan")
			err := os.WriteFile(planPath, nil, 0600)
			Ok(t, err)
			terraform := tfclientmocks.NewMockClient()
			step := runtime.ApplyStepRunner{
				TerraformExecutor: terraform,
			}

			output, err := step.Run(command.ProjectContext{
				Log:                logger,
				Workspace:          "workspace",
				RepoRelDir:         ".",
				EscapedCommentArgs: c.commentFlags,
			}, c.extraArgs, tmpDir, map[string]string(nil))
			Equals(t, "", output)
			if c.expErr {
				ErrEquals(t, "cannot run apply with -target because we are applying an already generated plan. Instead, run -target with atlantis plan", err)
			} else {
				Ok(t, err)
			}
		})
	}
}

// Test that apply works for remote applies.
func TestRun_RemoteApply_Success(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "workspace.tfplan")
	planFileContents := `
An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  - destroy

Terraform will perform the following actions:

  - null_resource.hi[1]


Plan: 0 to add, 0 to change, 1 to destroy.`
	err := os.WriteFile(planPath, []byte("Atlantis: this plan was created by remote ops\n"+planFileContents), 0600)
	Ok(t, err)

	RegisterMockTestingT(t)
	tfOut := fmt.Sprintf(preConfirmOutFmt, planFileContents) + postConfirmOut
	tfExec := &remoteApplyMock{LinesToSend: tfOut, DoneCh: make(chan bool)}
	updater := runtimemocks.NewMockStatusUpdater()
	o := runtime.ApplyStepRunner{
		AsyncTFExec:         tfExec,
		CommitStatusUpdater: updater,
	}
	tfVersion, _ := version.NewVersion("0.11.0")
	ctx := command.ProjectContext{
		Log:                logging.NewNoopLogger(t),
		Workspace:          "workspace",
		RepoRelDir:         ".",
		EscapedCommentArgs: []string{"comment", "args"},
		TerraformVersion:   tfVersion,
	}
	output, err := o.Run(ctx, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	<-tfExec.DoneCh

	Ok(t, err)
	Equals(t, "yes\n", tfExec.PassedInput)
	Equals(t, `
2019/02/27 21:47:36 [DEBUG] Using modified User-Agent: Terraform/0.11.11 TFE/d161c1b
null_resource.dir2[1]: Destroying... (ID: 8554368366766418126)
null_resource.dir2[1]: Destruction complete after 0s

Apply complete! Resources: 0 added, 0 changed, 1 destroyed.
`, output)

	Equals(t, []string{"apply", "-input=false", "-no-color", "extra", "args", "comment", "args"}, tfExec.CalledArgs)
	_, err = os.Stat(planPath)
	Assert(t, os.IsNotExist(err), "planfile should be deleted")

	// Check that the status was updated with the run url.
	runURL := "https://app.terraform.io/app/lkysow-enterprises/atlantis-tfe-test-dir2/runs/run-PiDsRYKGcerTttV2"
	updater.VerifyWasCalledOnce().UpdateProject(ctx, command.Apply, models.PendingCommitStatus, runURL, nil)
	updater.VerifyWasCalledOnce().UpdateProject(ctx, command.Apply, models.SuccessCommitStatus, runURL, nil)
}

// Test that if the plan is different, we error out.
func TestRun_RemoteApply_PlanChanged(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "workspace.tfplan")
	planFileContents := `
An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  - destroy

Terraform will perform the following actions:

  - null_resource.hi[1]


Plan: 0 to add, 0 to change, 1 to destroy.`
	err := os.WriteFile(planPath, []byte("Atlantis: this plan was created by remote ops\n"+planFileContents), 0600)
	Ok(t, err)

	RegisterMockTestingT(t)
	tfOut := fmt.Sprintf(preConfirmOutFmt, "not the expected plan!") + noConfirmationOut
	tfExec := &remoteApplyMock{
		LinesToSend: tfOut,
		Err:         errors.New("exit status 1"),
		DoneCh:      make(chan bool),
	}
	o := runtime.ApplyStepRunner{
		AsyncTFExec:         tfExec,
		CommitStatusUpdater: runtimemocks.NewMockStatusUpdater(),
	}
	tfVersion, _ := version.NewVersion("0.11.0")

	output, err := o.Run(command.ProjectContext{
		Log:                logging.NewNoopLogger(t),
		Workspace:          "workspace",
		RepoRelDir:         ".",
		EscapedCommentArgs: []string{"comment", "args"},
		TerraformVersion:   tfVersion,
	}, []string{"extra", "args"}, tmpDir, map[string]string(nil))
	<-tfExec.DoneCh
	ErrEquals(t, `Plan generated during apply phase did not match plan generated during plan phase.
Aborting apply.

Expected Plan:

An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  - destroy

Terraform will perform the following actions:

  - null_resource.hi[1]


Plan: 0 to add, 0 to change, 1 to destroy.
**************************************************

Actual Plan:

not the expected plan!
**************************************************

This likely occurred because someone applied a change to this state in-between
your plan and apply commands.
To resolve, re-run plan.`, err)
	Equals(t, "", output)
	Equals(t, "no\n", tfExec.PassedInput)

	// Planfile should not be deleted.
	_, err = os.Stat(planPath)
	Ok(t, err)
}

type remoteApplyMock struct {
	// LinesToSend will be sent on the channel.
	LinesToSend string
	// Err will be sent on the channel after all LinesToSend.
	Err error
	// CalledArgs is what args we were called with.
	CalledArgs []string
	// PassedInput is set to the last string passed to our input channel.
	PassedInput string
	// DoneCh callers should wait on the done channel to ensure we're done.
	DoneCh chan bool
}

// RunCommandAsync fakes out running terraform async.
func (r *remoteApplyMock) RunCommandAsync(_ command.ProjectContext, _ string, args []string, _ map[string]string, _ tf.Distribution, _ *version.Version, _ string) (chan<- string, <-chan runtimemodels.Line) {
	r.CalledArgs = args

	in := make(chan string)
	out := make(chan runtimemodels.Line)

	// We use a wait group to ensure our sending and receiving routines have
	// completed.
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		wg.Wait()
		// When they're done, we signal the done channel.
		r.DoneCh <- true
	}()

	// Asynchronously process input.
	go func() {
		inLine := <-in
		r.PassedInput = inLine
		close(in)
		wg.Done()
	}()

	// Asynchronously send the lines we're supposed to.
	go func() {
		for _, line := range strings.Split(r.LinesToSend, "\n") {
			out <- runtimemodels.Line{Line: line}
		}
		if r.Err != nil {
			out <- runtimemodels.Line{Err: r.Err}
		}
		close(out)
		wg.Done()
	}()
	return in, out
}

var preConfirmOutFmt = `
Running apply in the remote backend. Output will stream here. Pressing Ctrl-C
will cancel the remote apply if its still pending. If the apply started it
will stop streaming the logs, but will not stop the apply running remotely.

Preparing the remote apply...

To view this run in a browser, visit:
https://app.terraform.io/app/lkysow-enterprises/atlantis-tfe-test-dir2/runs/run-PiDsRYKGcerTttV2

Waiting for the plan to start...

Terraform v0.11.11

Configuring remote state backend...
Initializing Terraform configuration...
2019/02/27 21:50:44 [DEBUG] Using modified User-Agent: Terraform/0.11.11 TFE/d161c1b
Refreshing Terraform state in-memory prior to plan...
The refreshed state will be used to calculate this plan, but will not be
persisted to local or remote state storage.

null_resource.dir2[0]: Refreshing state... (ID: 8492616078576984857)

------------------------------------------------------------------------
%s

Do you want to perform these actions in workspace "atlantis-tfe-test-dir2"?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: `

var postConfirmOut = `

2019/02/27 21:47:36 [DEBUG] Using modified User-Agent: Terraform/0.11.11 TFE/d161c1b
null_resource.dir2[1]: Destroying... (ID: 8554368366766418126)
null_resource.dir2[1]: Destruction complete after 0s

Apply complete! Resources: 0 added, 0 changed, 1 destroyed.
`

var noConfirmationOut = `

Error: Apply discarded.
`
