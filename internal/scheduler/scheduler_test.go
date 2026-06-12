package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler(nil)

	t.Run("start and stop scheduler", func(t *testing.T) {
		s.Start()
		// Should not panic or error
		s.Stop()
	})

	t.Run("double start does not panic", func(t *testing.T) {
		s.Start()
		s.Start() // should be safe
		s.Stop()
	})

	t.Run("stop without start does not panic", func(t *testing.T) {
		s2 := NewScheduler(nil)
		s2.Stop() // should be safe
	})
}

func TestScheduler_RegisterUnregister(t *testing.T) {
	var count atomic.Int32
	backupFn := func(repoID string) error {
		count.Add(1)
		return nil
	}

	s := NewScheduler(backupFn)
	s.Start()
	defer s.Stop()

	t.Run("register a cron job", func(t *testing.T) {
		// Every second
		err := s.Register("repo-1", "* * * * * *")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("register with invalid cron expression", func(t *testing.T) {
		err := s.Register("repo-2", "invalid cron")
		if err == nil {
			t.Fatal("expected error for invalid cron expression")
		}
	})

	t.Run("unregister existing entry", func(t *testing.T) {
		s.Unregister("repo-1")
		// Should not panic
	})

	t.Run("unregister non-existent entry", func(t *testing.T) {
		s.Unregister("non-existent")
		// Should not panic
	})
}

func TestScheduler_JobExecution(t *testing.T) {
	var count atomic.Int32
	backupFn := func(repoID string) error {
		count.Add(1)
		return nil
	}

	s := NewScheduler(backupFn)
	s.Start()
	defer s.Stop()

	err := s.Register("test-repo", "* * * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for ~2 seconds to allow at least one execution
	time.Sleep(1500 * time.Millisecond)

	if count.Load() == 0 {
		t.Fatal("expected at least one job execution")
	}
}

func TestScheduler_BackupFnNil(t *testing.T) {
	s := NewScheduler(nil)
	s.Start()
	defer s.Stop()

	// Registering a job with nil backupFn should not cause issues
	// Jobs will be scheduled but no-op when executed
	err := s.Register("repo-nil", "* * * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}

func TestScheduler_ReRegister(t *testing.T) {
	var count atomic.Int32
	backupFn := func(repoID string) error {
		count.Add(1)
		return nil
	}

	s := NewScheduler(backupFn)
	s.Start()
	defer s.Stop()

	// Register with slow cron (6-field format with seconds)
	err := s.Register("repo-rereg", "0 0 1 1 1 *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-register with fast cron (should replace the old entry)
	err = s.Register("repo-rereg", "* * * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	if count.Load() == 0 {
		t.Fatal("expected job execution after re-register")
	}
}
