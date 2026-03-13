package scheduler

import (
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	Cron *cron.Cron
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		Cron: cron.New(),
	}
}

// Start begins the cron scheduler
func (s *Scheduler) Start() {
	s.Cron.Start()
	fmt.Println("Scheduler started")
}

// Stop halts the cron scheduler
func (s *Scheduler) Stop() {
	s.Cron.Stop()
	fmt.Println("Scheduler stopped")
}

// ScheduleFridayNightJob adds a job to run every Friday at 18:00 (6:00 PM)
// The cron expression "0 18 * * 5" means:
// 0 = minute 0
// 18 = 18th hour (6 PM)
// * = any day of month
// * = any month
// 5 = Friday
func (s *Scheduler) ScheduleFridayNightJob(job func()) error {
	_, err := s.Cron.AddFunc("0 18 * * 5", func() {
		log.Println("Starting Friday Night Movie job...")
		job()
		log.Println("Finished Friday Night Movie job.")
	})
	return err
}

// TriggerNow allows manually triggering the job
func (s *Scheduler) TriggerNow(job func()) {
	log.Println("Manually triggered Friday Night Movie job...")
	job()
	log.Println("Finished manual Friday Night Movie job.")
}

// NextRun returns the time of the next scheduled run
func (s *Scheduler) NextRun() string {
	entries := s.Cron.Entries()
	if len(entries) > 0 {
		return entries[0].Next.Format("Monday at 3:04 PM")
	}
	return "Not scheduled"
}
