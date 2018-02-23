package main

import (
	"flag"
	"fmt"

	"github.com/containerd/containerd/namespaces"
	"github.com/docker/distribution/reference"
	units "github.com/docker/go-units"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/jessfraz/img/source/containerimage"
	"github.com/jessfraz/img/worker/runc"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/appcontext"
)

const pullHelp = `Pull an image or a repository from a registry.`

func (cmd *pullCommand) Name() string      { return "pull" }
func (cmd *pullCommand) Args() string      { return "[OPTIONS] NAME[:TAG|@DIGEST]" }
func (cmd *pullCommand) ShortHelp() string { return pullHelp }
func (cmd *pullCommand) LongHelp() string  { return pullHelp }
func (cmd *pullCommand) Hidden() bool      { return false }

func (cmd *pullCommand) Register(fs *flag.FlagSet) {}

type pullCommand struct {
	image string
}

func (cmd *pullCommand) Run(args []string) (err error) {
	if len(args) < 1 {
		return fmt.Errorf("must pass an image or repository to pull")
	}

	// Get the specified image.
	cmd.image = args[0]

	// Create the context.
	ctx := appcontext.Context()
	id := identity.NewID()
	ctx = session.NewContext(ctx, id)
	ctx = namespaces.WithNamespace(ctx, namespaces.Default)

	// Parse the repository name.
	image, err := reference.ParseNormalizedNamed(cmd.image)
	if err != nil {
		return fmt.Errorf("not a valid image %q: %v", cmd.image, err)
	}
	// Add latest to the image name if it is empty.
	image = reference.TagNameOnly(image)

	// Get the identifier for the image.
	identifier, err := source.NewImageIdentifier(image.String())
	if err != nil {
		return err
	}

	// Create the source manager.
	sm, fuseserver, err := createSouceManager()
	defer unmount(fuseserver)
	if err != nil {
		return err
	}
	handleSignals(fuseserver)

	// Resolve (ie. pull) the image.
	si, err := sm.Resolve(ctx, identifier)
	if err != nil {
		return err
	}

	fmt.Printf("Pulling %s...\n", cmd.image)

	// Snapshot the image.
	ref, err := si.Snapshot(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Snapshot ref: %s\n", ref.ID())

	// Get the size.
	size, err := ref.Size(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Size: %s\n", units.BytesSize(float64(size)))

	return nil
}

func createSouceManager() (*source.Manager, *fuse.Server, error) {
	// Create the runc worker.
	opt, fuseserver, err := runc.NewWorkerOpt(stateDir, backend)
	if err != nil {
		return nil, fuseserver, fmt.Errorf("creating runc worker opt failed: %v", err)
	}

	cm, err := cache.NewManager(cache.ManagerOpt{
		Snapshotter:   opt.Snapshotter,
		MetadataStore: opt.MetadataStore,
	})
	if err != nil {
		return nil, fuseserver, err
	}

	sm, err := source.NewManager()
	if err != nil {
		return nil, fuseserver, err
	}

	is, err := containerimage.NewSource(containerimage.SourceOpt{
		Snapshotter:   opt.Snapshotter,
		ContentStore:  opt.ContentStore,
		Applier:       opt.Applier,
		CacheAccessor: cm,
	})
	if err != nil {
		return nil, fuseserver, err
	}

	sm.Register(is)

	return sm, fuseserver, nil
}
