package utils

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/reaandrew/techdetector/core"
	"math"
	"sort"
	"strings"
	"time"
)

func CollectGitMetrics(repoPath, repoName string) ([]core.Finding, error) {
	var findings []core.Finding

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	// Collect branch-related metrics
	branchFindings, err := getBranchMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, branchFindings...)

	// Collect tag-related metrics
	tagFindings, err := getTagMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, tagFindings...)

	// Collect commit-related metrics
	commitFindings, err := getCommitMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, commitFindings...)

	// Collect additional branch-based metrics
	branchActivityFindings, err := getBranchActivityMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, branchActivityFindings...)

	// Collect additional object size metrics
	objectSizeFindings, err := getObjectSizeMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, objectSizeFindings...)

	// For example, 2 MB threshold
	const MB = 1024 * 1024
	sizeThreshold := int64(2 * MB)

	oversizedFindings, err := getOversizedObjectFindings(repo, repoName, sizeThreshold)
	if err != nil {
		return nil, err
	}
	findings = append(findings, oversizedFindings...)

	// Example: skip merges, process up to 1000 commits
	opts := DiffStatsOptions{
		SkipMerges: true,
		MaxCommits: 1000,
	}

	// 1) Gather per-file stats
	fileStatsMap, err := collectPerFileChangeStats(repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to collect file stats: %w", err)
	}
	if len(fileStatsMap) == 0 {
		// Possibly no commits or all merges were skipped
		return findings, nil
	}

	// 2) Derive top-level metrics & produce findings
	diffFindings := createFileDiffFindings(repoName, fileStatsMap)
	findings = append(findings, diffFindings...)

	compressedFindings, err := getCompressedFileAndLineMetrics(repo, repoName, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to collect compressed file change metrics: %w", err)
	}
	findings = append(findings, compressedFindings...)

	return findings, nil
}

// ----------------- Branch Metrics -----------------
func getBranchMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding
	branches, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve branches: %w", err)
	}

	var branchCount int
	var branchNames []string

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() || ref.Name().IsBranch() {
			branchNames = append(branchNames, ref.Name().String())
			branchCount++
		}
		return nil
	})

	findings = append(findings, core.Finding{
		Name:     "Branch Count",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value":    branchCount,
			"branches": branchNames,
		},
		RepoName: repoName,
	})

	return findings, nil
}

// ----------------- Tag Metrics -----------------
func getTagMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding
	var tagDates []time.Time

	// Iterate over all tags (includes lightweight and annotated)
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tags: %w", err)
	}

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tag, err := repo.TagObject(ref.Hash())
		if err == nil {
			// Annotated tag
			tagDates = append(tagDates, tag.Tagger.When)
		} else {
			// Lightweight tag - get commit date directly
			commit, err := repo.CommitObject(ref.Hash())
			if err == nil {
				tagDates = append(tagDates, commit.Committer.When)
			}
		}
		return nil
	})

	// Calculate average time between tags
	if len(tagDates) > 1 {
		sort.Slice(tagDates, func(i, j int) bool {
			return tagDates[i].Before(tagDates[j])
		})

		var totalTime time.Duration
		for i := 1; i < len(tagDates); i++ {
			totalTime += tagDates[i].Sub(tagDates[i-1])
		}

		averageTagTime := totalTime / time.Duration(len(tagDates)-1)

		findings = append(findings, core.Finding{
			Name:     "Average Time Between Tags",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"value": fmt.Sprintf("%.2f days", averageTagTime.Hours()/24),
			},
			RepoName: repoName,
		})
	} else {
		findings = append(findings, core.Finding{
			Name:     "Average Time Between Tags",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"value": "Not enough tags to calculate",
			},
			RepoName: repoName,
		})
	}

	return findings, nil
}

// ----------------- Commit Metrics -----------------
func getCommitMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding

	commitIter, err := repo.Log(&git.LogOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit history: %w", err)
	}

	commitsPerDay := make(map[string]int)
	commitsPerWeek := make(map[string]int)
	commitsPerMonth := make(map[string]int)

	var firstCommitDate, lastCommitDate time.Time
	var totalCommits int
	authorSet := make(map[string]struct{})

	err = commitIter.ForEach(func(c *object.Commit) error {
		commitDate := c.Committer.When

		// Fixing the ISOWeek error by capturing both return values
		year, week := commitDate.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%d", year, week)
		monthKey := commitDate.Format("2006-01")
		dayKey := commitDate.Format("2006-01-02")

		commitsPerDay[dayKey]++
		commitsPerWeek[weekKey]++
		commitsPerMonth[monthKey]++
		authorSet[c.Author.Email] = struct{}{}

		if firstCommitDate.IsZero() || commitDate.Before(firstCommitDate) {
			firstCommitDate = commitDate
		}
		if commitDate.After(lastCommitDate) {
			lastCommitDate = commitDate
		}

		totalCommits++
		return nil
	})

	maxCommitsPerDay, avgCommitsPerDay := calculateMaxAndAvg(commitsPerDay)
	maxCommitsPerWeek, avgCommitsPerWeek := calculateMaxAndAvg(commitsPerWeek)
	maxCommitsPerMonth, avgCommitsPerMonth := calculateMaxAndAvg(commitsPerMonth)

	// Add findings
	findings = append(findings, core.Finding{
		Name:     "Total Commits",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": totalCommits,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "First Commit Date",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": firstCommitDate.Format(time.RFC3339),
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Last Commit Date",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": lastCommitDate.Format(time.RFC3339),
		},
		RepoName: repoName,
	})

	// Add max and average commits per day/week/month to findings
	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Day",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerDay,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Day",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerDay,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Week",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerWeek,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Week",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerWeek,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Month",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerMonth,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Month",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerMonth,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Unique Contributors",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": len(authorSet),
		},
		RepoName: repoName,
	})

	return findings, nil
}

// ----------------- Utility Function -----------------
func calculateMaxAndAvg(commitStats map[string]int) (int, float64) {
	var maxCommits int
	var totalCommits int

	for _, count := range commitStats {
		totalCommits += count
		if count > maxCommits {
			maxCommits = count
		}
	}

	avgCommits := 0.0
	if len(commitStats) > 0 {
		avgCommits = float64(totalCommits) / float64(len(commitStats))
	}

	return maxCommits, avgCommits
}

// ----------------- Branch Activity Metrics -----------------
func getBranchActivityMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding

	refIter, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve branch references: %w", err)
	}

	var latestCommitDate time.Time
	minDaysSinceLastCommit := math.MaxInt64
	totalCommits := 0
	branchCommitCounts := make(map[string]int)

	forEachErr := refIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() || ref.Name().IsBranch() {
			branchName := ref.Name().Short()
			commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
			if err != nil {
				return fmt.Errorf("failed to retrieve commits for branch %s: %w", branchName, err)
			}

			commitCount := 0
			var lastCommitDate time.Time
			commitIter.ForEach(func(c *object.Commit) error {
				commitCount++
				if c.Committer.When.After(lastCommitDate) {
					lastCommitDate = c.Committer.When
				}
				return nil
			})

			branchCommitCounts[branchName] = commitCount
			totalCommits += commitCount

			// Check for latest commit
			if lastCommitDate.After(latestCommitDate) {
				latestCommitDate = lastCommitDate
			}

			// Calculate days since last commit
			daysSinceLastCommit := int(time.Since(lastCommitDate).Hours() / 24)
			if daysSinceLastCommit < minDaysSinceLastCommit {
				minDaysSinceLastCommit = daysSinceLastCommit
			}
		}
		return nil
	})

	if forEachErr != nil {
		return nil, forEachErr
	}

	// Calculate days since last commit on default branch (assume "main")
	mainRef, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err == nil {
		commitIter, _ := repo.Log(&git.LogOptions{From: mainRef.Hash()})
		mainLastCommit, _ := commitIter.Next()
		mainDaysSinceLastCommit := int(time.Since(mainLastCommit.Committer.When).Hours() / 24)

		findings = append(findings, core.Finding{
			Name:     "Days Since Last Commit to Main Branch",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"value": mainDaysSinceLastCommit,
			},
			RepoName: repoName,
		})
	}

	// Store the minimum days since last commit across all branches
	findings = append(findings, core.Finding{
		Name:     "Days Since Last Commit to Any Branch",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": minDaysSinceLastCommit,
		},
		RepoName: repoName,
	})

	// Calculate average commits per branch
	avgCommitsPerBranch := float64(totalCommits) / float64(len(branchCommitCounts))
	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Branch",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerBranch,
		},
		RepoName: repoName,
	})

	// Determine the branch with the max commits
	var maxCommitsBranch string
	maxCommits := 0
	for branch, count := range branchCommitCounts {
		if count > maxCommits {
			maxCommits = count
			maxCommitsBranch = branch
		}
	}

	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Branch",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value":       maxCommits,
			"branch_name": maxCommitsBranch,
		},
		RepoName: repoName,
	})

	return findings, nil
}

// getObjectSizeMetrics calculates the max and average size of all blob objects in the repo.
func getObjectSizeMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	// --- First Pass: Gather Overall Object Stats ---
	maxBlobHash, maxSize, avgSize, err := findMaxAndAvgBlobSize(repo)
	if err != nil {
		return nil, err
	}

	// --- Second Pass: Find One Reference for Largest Blob (if any) ---
	blobRef, err := findReferenceForBlob(repo, maxBlobHash)
	if err != nil {
		return nil, err
	}

	var findings []core.Finding

	// 1) Finding: Maximum Object Size (+ extra properties)
	maxSizeProps := map[string]interface{}{
		"value": fmt.Sprintf("%d bytes", maxSize),
	}
	if blobRef != nil {
		maxSizeProps["branch_name"] = blobRef.BranchName
		maxSizeProps["commit_hash"] = blobRef.CommitHash
		maxSizeProps["commit_date"] = blobRef.CommitDate.Format(time.RFC3339)
		maxSizeProps["filename"] = blobRef.FilePath
	}

	findings = append(findings, core.Finding{
		Name:       "Max Object Size",
		Type:       "git_metric",
		Category:   "repository_analysis",
		Properties: maxSizeProps,
		RepoName:   repoName,
	})

	// 2) Finding: Average Object Size (no single commit reference)
	findings = append(findings, core.Finding{
		Name:     "Average Object Size",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": fmt.Sprintf("%.2f bytes", avgSize),
		},
		RepoName: repoName,
	})

	return findings, nil
}

// findMaxAndAvgBlobSize enumerates all blob objects to find the largest blob and average size.
func findMaxAndAvgBlobSize(repo *git.Repository) (maxBlobHash plumbing.Hash, maxSize int64, avgSize float64, err error) {
	objIter, err := repo.Objects()
	if err != nil {
		return plumbing.Hash{}, 0, 0, fmt.Errorf("failed to open object iterator: %w", err)
	}
	defer objIter.Close()

	var totalSize int64
	var objectCount int64

	err = objIter.ForEach(func(o object.Object) error {
		// We only care about blobs
		blob, ok := o.(*object.Blob)
		if !ok {
			return nil
		}
		size := blob.Size
		totalSize += size
		objectCount++

		if size > maxSize {
			maxSize = size
			maxBlobHash = blob.Hash
		}
		return nil
	})
	if err != nil {
		return plumbing.Hash{}, 0, 0, err
	}

	if objectCount > 0 {
		avgSize = float64(totalSize) / float64(objectCount)
	}

	return maxBlobHash, maxSize, avgSize, nil
}

// BlobReference holds metadata about the largest blob's location.
type BlobReference struct {
	BranchName string
	CommitHash string
	CommitDate time.Time
	FilePath   string
}

func findReferenceForBlob(repo *git.Repository, targetHash plumbing.Hash) (*BlobReference, error) {
	if targetHash.IsZero() {
		// No largest blob found
		return nil, nil
	}

	refIter, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve references: %w", err)
	}
	defer refIter.Close()

	var found *BlobReference

	err = refIter.ForEach(func(ref *plumbing.Reference) error {
		// Skip if not a branch or remote
		if !(ref.Name().IsBranch() || ref.Name().IsRemote()) {
			return nil
		}

		branchName := ref.Name().Short()
		headCommit, commitErr := repo.CommitObject(ref.Hash())
		if commitErr != nil {
			return nil
		}

		tree, treeErr := headCommit.Tree()
		if treeErr != nil {
			return fmt.Errorf("failed to get tree for commit %s in branch %s: %w",
				headCommit.Hash, branchName, treeErr)
		}

		// Look for the target blob in the HEAD commit's tree
		walkErr := tree.Files().ForEach(func(file *object.File) error {
			if file.Blob.Hash == targetHash {
				// Found our blob
				found = &BlobReference{
					BranchName: branchName,
					CommitHash: headCommit.Hash.String(),
					CommitDate: headCommit.Committer.When,
					FilePath:   file.Name,
				}
				// Use storer.ErrStop to stop this Files().ForEach
				return storer.ErrStop
			}
			return nil
		})
		if walkErr != nil && walkErr != storer.ErrStop {
			return walkErr
		}

		// If we found it, stop the outer References().ForEach
		if found != nil {
			// Use storer.ErrStop to stop this RefIter
			return storer.ErrStop
		}
		return nil
	})
	// Distinguish between normal early stop and actual errors
	if err != nil && err != storer.ErrStop {
		return nil, err
	}

	return found, nil
}

// BigBlobRef is a data structure to link a blob to a single HEAD commit reference.
type BigBlobRef struct {
	BlobHash   plumbing.Hash
	BlobSize   int64
	BranchName string
	CommitHash string
	CommitDate time.Time
	FilePath   string
}

// getOversizedBlobs enumerates all blobs to find which exceed the given threshold.
func getOversizedBlobs(repo *git.Repository, sizeThreshold int64) ([]plumbing.Hash, map[plumbing.Hash]int64, error) {
	objIter, err := repo.Objects()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open object iterator: %w", err)
	}
	defer objIter.Close()

	var bigBlobs []plumbing.Hash
	blobSizes := make(map[plumbing.Hash]int64)

	err = objIter.ForEach(func(o object.Object) error {
		blob, ok := o.(*object.Blob)
		if !ok {
			return nil // Skip if not a blob
		}
		if blob.Size > sizeThreshold {
			bigBlobs = append(bigBlobs, blob.Hash)
			blobSizes[blob.Hash] = blob.Size
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return bigBlobs, blobSizes, nil
}

// findBlobReferencesInHeads tries to find references to the given blob hash
// in the HEAD commits of all branches. Returns zero or more references.
func findBlobReferencesInHeads(
	repo *git.Repository,
	targetBlob plumbing.Hash,
) ([]BigBlobRef, error) {

	var refsFound []BigBlobRef

	// If the target blob is zero, there's nothing to find
	if targetBlob.IsZero() {
		return refsFound, nil
	}

	refIter, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve references: %w", err)
	}
	defer refIter.Close()

	err = refIter.ForEach(func(ref *plumbing.Reference) error {
		if !(ref.Name().IsBranch() || ref.Name().IsRemote()) {
			return nil // Only look at branches or remote refs
		}

		branchName := ref.Name().Short()

		headCommit, commitErr := repo.CommitObject(ref.Hash())
		if commitErr != nil {
			return nil
		}

		tree, treeErr := headCommit.Tree()
		if treeErr != nil {
			return fmt.Errorf("failed to get tree for commit %s in branch %s: %w",
				headCommit.Hash, branchName, treeErr)
		}

		// Walk the files in the HEAD commit tree
		fileErr := tree.Files().ForEach(func(f *object.File) error {
			if f.Blob.Hash == targetBlob {
				refsFound = append(refsFound, BigBlobRef{
					BlobHash:   targetBlob,
					BranchName: branchName,
					CommitHash: headCommit.Hash.String(),
					CommitDate: headCommit.Committer.When,
					FilePath:   f.Name,
				})
				// We won't break here because the same blob might appear more than once in a tree if the file is duplicated (rare).
			}
			return nil
		})
		if fileErr != nil && fileErr != storer.ErrStop {
			return fileErr
		}

		return nil
	})
	if err != nil && err != storer.ErrStop {
		return nil, err
	}

	return refsFound, nil
}

// getOversizedObjectFindings is the main function that combines scanning and references.
func getOversizedObjectFindings(repo *git.Repository, repoName string, sizeThreshold int64) ([]core.Finding, error) {
	bigBlobs, blobSizes, err := getOversizedBlobs(repo, sizeThreshold)
	if err != nil {
		return nil, err
	}
	if len(bigBlobs) == 0 {
		// No large blobs found
		return nil, nil
	}

	var findings []core.Finding

	// For each discovered big blob, find references in HEAD
	for _, blobHash := range bigBlobs {
		size := blobSizes[blobHash]
		blobRefs, err := findBlobReferencesInHeads(repo, blobHash)
		if err != nil {
			return nil, fmt.Errorf("failed to find references for blob %s: %w", blobHash.String(), err)
		}

		// If no references found, it likely means the blob is orphaned (not in HEAD).
		if len(blobRefs) == 0 {
			// We might still want a single "orphaned" finding
			findings = append(findings, core.Finding{
				Name:     "Large Orphaned Blob",
				Type:     "git_metric",
				Category: "repository_analysis",
				Properties: map[string]interface{}{
					"blob_hash": blobHash.String(),
					"size":      fmt.Sprintf("%d bytes", size),
					"note":      "No HEAD references found (blob may be in older commits).",
				},
				RepoName: repoName,
			})
			continue
		}

		// Otherwise, create a separate finding for each reference found
		for _, br := range blobRefs {
			findings = append(findings, core.Finding{
				Name:     "Large Blob Found",
				Type:     "git_metric",
				Category: "repository_analysis",
				Properties: map[string]interface{}{
					"value":       fmt.Sprintf("%d bytes", size),
					"blob_hash":   br.BlobHash.String(),
					"branch_name": br.BranchName,
					"commit_hash": br.CommitHash,
					"commit_date": br.CommitDate.Format(time.RFC3339),
					"filename":    br.FilePath,
				},
				RepoName: repoName,
			})
		}
	}

	return findings, nil
}

// DiffStatsOptions toggles behaviour for scanning commits.
type DiffStatsOptions struct {
	SkipMerges bool // If true, skip multi-parent (merge) commits entirely.
	MaxCommits int  // If > 0, limit scanning to this many commits.
}

// FileStats accumulates stats for one file path across all scanned commits.
type FileStats struct {
	CommitsTouched           int
	TotalLinesAdded          int
	TotalLinesDeleted        int
	Contributors             map[string]struct{}
	MaxLinesAddedInOneCommit int
}

// newFileStats is a helper to initialise FileStats with a contributor set.
func newFileStats() *FileStats {
	return &FileStats{
		Contributors: make(map[string]struct{}),
	}
}

// collectPerFileChangeStats enumerates commits in the repo's log, computes diffs
// against each commit's parent, and aggregates line-level stats per file.
func collectPerFileChangeStats(repo *git.Repository, opts DiffStatsOptions) (map[string]*FileStats, error) {
	fileStatsMap := make(map[string]*FileStats)

	// By default, repo.Log() enumerates all commits reachable from HEAD (all branches).
	// If you only want a single branch, see below for how to scope it.
	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit log: %w", err)
	}
	defer commitIter.Close()

	commitsProcessed := 0

	err = commitIter.ForEach(func(commit *object.Commit) error {
		// Possibly skip merges if requested
		if opts.SkipMerges && commit.NumParents() > 1 {
			return nil
		}
		if opts.MaxCommits > 0 && commitsProcessed >= opts.MaxCommits {
			// We’ve hit the limit; stop iterating early
			return storer.ErrStop
		}
		commitsProcessed++

		// If there's no parent (e.g. first commit), or multiple parents (merge) we skip diff
		if commit.NumParents() != 1 {
			return nil
		}
		parent, err := commit.Parent(0)
		if err != nil {
			return nil
		}

		// Compare trees
		oldTree, err := parent.Tree()
		if err != nil {
			return nil
		}
		newTree, err := commit.Tree()
		if err != nil {
			return nil
		}

		// Diff returns object.Changes, not a patch directly
		changes, err := oldTree.Diff(newTree)
		if err != nil {
			return fmt.Errorf("failed generating diff for %s: %w", commit.Hash, err)
		}

		// Convert changes to a patch for per-file iteration
		patch, err := changes.Patch()
		if err != nil {
			return fmt.Errorf("failed to create patch for %s: %w", commit.Hash, err)
		}

		// For each file patch, we can see lines added/deleted
		for _, fp := range patch.FilePatches() {
			from, to := fp.Files()
			var fileName string
			switch {
			case to == nil:
				// file was deleted
				fileName = from.Path()
			case from == nil:
				// file was added
				fileName = to.Path()
			default:
				// file was modified or renamed
				fileName = to.Path()
			}

			stats, exists := fileStatsMap[fileName]
			if !exists {
				stats = newFileStats()
				fileStatsMap[fileName] = stats
			}

			// This file was changed in this commit
			stats.CommitsTouched++
			// Track the contributor by email
			stats.Contributors[commit.Author.Email] = struct{}{}

			added, deleted := countLinesAddedAndDeleted(fp)
			stats.TotalLinesAdded += added
			stats.TotalLinesDeleted += deleted

			if added > stats.MaxLinesAddedInOneCommit {
				stats.MaxLinesAddedInOneCommit = added
			}
		}

		return nil
	})
	// If we early-stopped or finished commits gracefully, storer.ErrStop is expected
	if err != nil && err != storer.ErrStop {
		return nil, err
	}

	return fileStatsMap, nil
}

// countLinesAddedAndDeleted counts how many lines were added/deleted within a single file patch
func countLinesAddedAndDeleted(fp diff.FilePatch) (int, int) {
	var totalAdded, totalDeleted int
	for _, chunk := range fp.Chunks() {
		switch chunk.Type() {
		case diff.Add:
			// Count newlines in chunk text
			totalAdded += strings.Count(chunk.Content(), "\n")
		case diff.Delete:
			totalDeleted += strings.Count(chunk.Content(), "\n")
		case diff.Equal:
			// no change
		}
	}
	return totalAdded, totalDeleted
}

// createFileDiffFindings takes the final map of per-file stats and produces separate findings:
// - File with Most Commits
// - File with Most Churn
// - File with Most Contributors
// - File with Max Single-Commit Lines Added
// - Average Lines Added/Deleted (per file)
func createFileDiffFindings(repoName string, fileStatsMap map[string]*FileStats) []core.Finding {
	if len(fileStatsMap) == 0 {
		// No data => no findings
		return nil
	}

	var mostCommitsFile string
	var mostCommits int

	var mostChurnFile string
	var mostChurn int

	var mostContribsFile string
	var mostContribs int

	var maxLinesAddedFile string
	var maxLinesAdded int

	// For "average lines" we do a simple total across all files
	var totalLinesAdded int
	var totalLinesDeleted int
	totalFiles := len(fileStatsMap)

	for fileName, st := range fileStatsMap {
		totalLinesAdded += st.TotalLinesAdded
		totalLinesDeleted += st.TotalLinesDeleted

		// Highest commit count
		if st.CommitsTouched > mostCommits {
			mostCommits = st.CommitsTouched
			mostCommitsFile = fileName
		}

		// Churn = lines added + lines deleted
		churn := st.TotalLinesAdded + st.TotalLinesDeleted
		if churn > mostChurn {
			mostChurn = churn
			mostChurnFile = fileName
		}

		// Contributors
		numContribs := len(st.Contributors)
		if numContribs > mostContribs {
			mostContribs = numContribs
			mostContribsFile = fileName
		}

		// Single-commit line additions
		if st.MaxLinesAddedInOneCommit > maxLinesAdded {
			maxLinesAdded = st.MaxLinesAddedInOneCommit
			maxLinesAddedFile = fileName
		}
	}

	avgLinesAdded := float64(totalLinesAdded) / float64(totalFiles)
	avgLinesDeleted := float64(totalLinesDeleted) / float64(totalFiles)

	// Now build discrete findings
	var findings []core.Finding

	// 1. File with Most Commits
	findings = append(findings, core.Finding{
		Name:     "File with Most Commits",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"filename":        mostCommitsFile,
			"commits_touched": mostCommits,
		},
		RepoName: repoName,
	})

	// 2. File with Most Churn
	findings = append(findings, core.Finding{
		Name:     "File with Most Churn",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"filename": mostChurnFile,
			"churn":    mostChurn,
		},
		RepoName: repoName,
	})

	// 3. File with Most Contributors
	findings = append(findings, core.Finding{
		Name:     "File with Most Contributors",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"filename":     mostContribsFile,
			"contributors": mostContribs,
		},
		RepoName: repoName,
	})

	// 4. File with Max Single-Commit Lines Added
	findings = append(findings, core.Finding{
		Name:     "File with Max Single-Commit Lines Added",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"filename":        maxLinesAddedFile,
			"max_lines_added": maxLinesAdded,
		},
		RepoName: repoName,
	})

	// 5. Average Lines Added / Deleted
	findings = append(findings, core.Finding{
		Name:     "Average Lines Added (Per File)",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgLinesAdded,
		},
		RepoName: repoName,
	})
	findings = append(findings, core.Finding{
		Name:     "Average Lines Deleted (Per File)",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgLinesDeleted,
		},
		RepoName: repoName,
	})

	return findings
}

type FileChangeStats struct {
	Daily   map[string]ChangeMetrics
	Weekly  map[string]ChangeMetrics
	Monthly map[string]ChangeMetrics
}

// ChangeMetrics stores file-level and line-level changes.
type ChangeMetrics struct {
	FilesAdded   int
	FilesDeleted int
	LinesAdded   int
	LinesDeleted int
}

// Collect file + line changes for day/week/month
func collectFileAndLineStats(repo *git.Repository, maxCommits int) (*FileChangeStats, error) {
	// Prepare stats maps
	fileCounts := &FileChangeStats{
		Daily:   make(map[string]ChangeMetrics),
		Weekly:  make(map[string]ChangeMetrics),
		Monthly: make(map[string]ChangeMetrics),
	}

	// Retrieve all commits (all branches/refs)
	commitIter, err := repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit log: %w", err)
	}
	defer commitIter.Close()

	commitCounter := 0

	err = commitIter.ForEach(func(commit *object.Commit) error {
		if maxCommits > 0 && commitCounter >= maxCommits {
			return storer.ErrStop
		}
		commitCounter++

		commitDate := commit.Committer.When.UTC()
		dayKey := commitDate.Format("20060102")
		year, week := commitDate.ISOWeek()
		weekKey := fmt.Sprintf("%dW%02d", year, week)
		monthKey := commitDate.Format("200601")

		// Root commit logic
		if commit.NumParents() == 0 {
			currentTree, err := commit.Tree()
			if err != nil {
				return fmt.Errorf("failed to get tree for root commit: %w", err)
			}
			err = currentTree.Files().ForEach(func(file *object.File) error {
				// For root commits, treat each file as fully "added"
				// We can't detect line-level changes for the root beyond "all lines are added" (optional).
				// We'll just increment FilesAdded by 1.
				stats := fileCounts.getOrCreate(dayKey, weekKey, monthKey)

				// Increase file-level add
				stats.FilesAdded++

				// If you wanted to approximate lines for the root commit, you could read the file size or do a line count:
				lines, err := file.Lines()
				if err != nil {
					return err
				}

				// lines is a []string, so to get the count:
				stats.LinesAdded += len(lines)

				fileCounts.set(dayKey, weekKey, monthKey, stats)
				return nil
			})
			return err
		}

		// Normal commits: Compare with first parent
		parent, err := commit.Parent(0)
		if err != nil {
			return nil // skip commits we can't read parent for
		}

		parentTree, err := parent.Tree()
		if err != nil {
			return fmt.Errorf("failed to get parent tree: %w", err)
		}
		currentTree, err := commit.Tree()
		if err != nil {
			return fmt.Errorf("failed to get current tree: %w", err)
		}

		// Identify file-level changes via merkletrie
		changes, err := parentTree.Diff(currentTree)
		if err != nil {
			return fmt.Errorf("failed to compute diff: %w", err)
		}

		// For line-level changes, get a Patch() that we can chunk-scan
		patch, err := changes.Patch()
		if err != nil {
			return fmt.Errorf("failed to create patch: %w", err)
		}

		var totalLinesAdded, totalLinesDeleted int

		for _, change := range changes {
			action, _ := change.Action()

			stats := fileCounts.getOrCreate(dayKey, weekKey, monthKey)
			switch action {
			case merkletrie.Insert:
				stats.FilesAdded++
			case merkletrie.Delete:
				stats.FilesDeleted++
			}
			fileCounts.set(dayKey, weekKey, monthKey, stats)
		}

		// Now gather line-level changes from the patch
		for _, fp := range patch.FilePatches() {
			for _, chunk := range fp.Chunks() {
				switch chunk.Type() {
				case diff.Add:
					countAdded := strings.Count(chunk.Content(), "\n")
					totalLinesAdded += countAdded
				case diff.Delete:
					countDeleted := strings.Count(chunk.Content(), "\n")
					totalLinesDeleted += countDeleted
				}
			}
		}

		// Add line-level totals for the day/week/month
		stats := fileCounts.getOrCreate(dayKey, weekKey, monthKey)
		stats.LinesAdded += totalLinesAdded
		stats.LinesDeleted += totalLinesDeleted
		fileCounts.set(dayKey, weekKey, monthKey, stats)

		return nil
	})

	if err != nil && err != storer.ErrStop {
		return nil, fmt.Errorf("error iterating commits: %w", err)
	}

	return fileCounts, nil
}

// Helper to get or create the stats struct in each map
func (f *FileChangeStats) getOrCreate(dayKey, weekKey, monthKey string) ChangeMetrics {
	dayStats := f.Daily[dayKey]
	return dayStats
}

func (f *FileChangeStats) set(dayKey, weekKey, monthKey string, stats ChangeMetrics) {
	f.Daily[dayKey] = stats

	// The same logic to sync to weekly, monthly:
	weekStats := f.Weekly[weekKey]
	weekStats.FilesAdded += stats.FilesAdded - weekStats.FilesAdded
	weekStats.FilesDeleted += stats.FilesDeleted - weekStats.FilesDeleted
	weekStats.LinesAdded += stats.LinesAdded - weekStats.LinesAdded
	weekStats.LinesDeleted += stats.LinesDeleted - weekStats.LinesDeleted
	f.Weekly[weekKey] = weekStats

	monthStats := f.Monthly[monthKey]
	monthStats.FilesAdded += stats.FilesAdded - monthStats.FilesAdded
	monthStats.FilesDeleted += stats.FilesDeleted - monthStats.FilesDeleted
	monthStats.LinesAdded += stats.LinesAdded - monthStats.LinesAdded
	monthStats.LinesDeleted += stats.LinesDeleted - monthStats.LinesDeleted
	f.Monthly[monthKey] = monthStats
}

func compressChangeStats(stats map[string]ChangeMetrics) []string {
	compressed := make([]string, 0, len(stats))

	for period, cm := range stats {
		line := fmt.Sprintf("%s:%d:%d:%d:%d",
			period,
			cm.FilesAdded,
			cm.FilesDeleted,
			cm.LinesAdded,
			cm.LinesDeleted,
		)
		compressed = append(compressed, line)
	}

	sort.Strings(compressed) // Sort to maintain chronological or lexical order
	return compressed
}

func getCompressedFileAndLineMetrics(repo *git.Repository, repoName string, maxCommits int) ([]core.Finding, error) {
	stats, err := collectFileAndLineStats(repo, maxCommits)
	if err != nil {
		return nil, err
	}

	dailyCompressed := compressChangeStats(stats.Daily)     // e.g. ["20231204:1:0:15:0", ...]
	weeklyCompressed := compressChangeStats(stats.Weekly)   // e.g. ["2023W49:4:1:30:10", ...]
	monthlyCompressed := compressChangeStats(stats.Monthly) // e.g. ["202312:15:2:60:20", ...]

	return []core.Finding{
		{
			Name:     "Daily File+Line Change (Compressed)",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"changes": dailyCompressed,
			},
			RepoName: repoName,
		},
		{
			Name:     "Weekly File+Line Change (Compressed)",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"changes": weeklyCompressed,
			},
			RepoName: repoName,
		},
		{
			Name:     "Monthly File+Line Change (Compressed)",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"changes": monthlyCompressed,
			},
			RepoName: repoName,
		},
	}, nil
}
