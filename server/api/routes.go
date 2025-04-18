package api

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/antonlindstrom/pgstore"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cskr/pubsub"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/CarsonHoffman/office-hours-queue/server/config"
)

type Server struct {
	chi.Router

	logger       *zap.SugaredLogger
	sessions     *pgstore.PGStore
	ps           *pubsub.PubSub
	oauthConfig  oauth2.Config
	oidcProvider *oidc.Provider

	// The number of WebSockets connected to each queue.
	websocketCount        map[ksuid.KSUID]int
	websocketCountByEmail map[ksuid.KSUID]map[string]int
	websocketCountLock    sync.Mutex
}

// All of the abilities that a complete backing
// store for the queue should have.
type queueStore interface {
	transactioner

	siteAdmin
	courseAdmin
	getUserInfo

	getCourses
	getCourse
	getAdminCourses
	addCourse
	updateCourse
	deleteCourse
	getCourseAdmins
	addCourseAdmins
	removeCourseAdmins

	getQueues
	getQueue
	addQueue
	updateQueue
	removeQueue
	getQueueEntry
	getQueueEntries
	addQueueEntry
	updateQueueEntry
	randomizeQueueEntries
	clearQueueEntries
	removeQueueEntry
	pinQueueEntry
	setQueueEntryHelping
	getQueueStack
	getQueueAnnouncements
	addQueueAnnouncement
	removeQueueAnnouncement
	getCurrentDaySchedule
	getQueueSchedule
	updateQueueSchedule
	getQueueConfiguration
	updateQueueConfiguration
	updateQueueOpenStatus
	getQueueRoster
	getQueueGroups
	updateQueueGroups
	setNotHelped
	queueStats

	getAppointment
	getAppointments
	getAppointmentsForUser
	getAppointmentsByTimeslot
	getAppointmentSchedule
	getAppointmentScheduleForDay
	updateAppointmentSchedule
	claimTimeslot
	unclaimAppointment
	signupForAppointment
	updateAppointment
	removeAppointmentSignup
}

func New(q queueStore, logger *zap.SugaredLogger, sessionsStore *sql.DB, oidcProvider *oidc.Provider, oauthConfig oauth2.Config) *Server {
	var s Server
	s.websocketCount = make(map[ksuid.KSUID]int)
	s.websocketCountByEmail = make(map[ksuid.KSUID]map[string]int)
	s.logger = logger

	var err error
	s.sessions, err = pgstore.NewPGStoreFromPool(sessionsStore, config.AppConfig.SessionsKey)
	if err != nil {
		logger.Fatalw("couldn't set up session store", "err", err)
	}
	s.sessions.Options = &sessions.Options{
		HttpOnly: true,
		Secure:   config.AppConfig.UseSecureCookies,
		MaxAge:   60 * 60 * 24 * 30,
		Path:     "/",
	}

	// TODO: evaluate capacity choice for channel. This assumes that
	// there isn't likely to be more than 5 events in "quick" succession
	// to any particular connection, and reduces overall latency between
	// sending on different connections in that case, but allocates room
	// for 5 events on every connection. There isn't an empirical basis here.
	// Just a guess.
	s.ps = pubsub.New(5)

	s.oauthConfig = oauthConfig
	s.oidcProvider = oidcProvider

	s.Router = chi.NewRouter()
	s.Router.Use(instrumenter, ksuidInserter, s.realIPOrFail, s.setupCtxLogger, s.recoverMiddleware, s.transaction(q), s.sessionRetriever)

	// Course endpoints
	s.Route("/courses", func(r chi.Router) {
		// Get all courses
		r.Method("GET", "/", s.GetCourses(q))

		// Create course (site admin)
		r.With(s.ValidLoginMiddleware, s.EnsureSiteAdmin(q, true), s.rateLimiter(5, time.Minute)).Method("POST", "/", s.AddCourse(q))

		// Course by ID endpoints
		r.Route("/{id:[a-zA-Z0-9]{27}}", func(r chi.Router) {
			r.Use(s.CourseIDMiddleware(q))

			// Get course by ID
			r.Method("GET", "/", s.GetCourse(q))

			// Get course's queues
			r.Method("GET", "/queues", s.GetQueues(q))

			// Update course (course admin)
			r.With(s.ValidLoginMiddleware, s.CheckCourseAdmin(q), s.EnsureCourseAdmin).Method("PUT", "/", s.UpdateCourse(q))

			r.With(s.ValidLoginMiddleware, s.CheckCourseAdmin(q), s.EnsureCourseAdmin).Method("DELETE", "/", s.DeleteCourse(q))

			// Create queue on course (course admin)
			r.With(s.ValidLoginMiddleware, s.CheckCourseAdmin(q), s.EnsureCourseAdmin, s.rateLimiter(5, time.Minute)).Method("POST", "/queues", s.AddQueue(q))

			// Course admin management (course admin)
			r.Route("/admins", func(r chi.Router) {
				r.Use(s.ValidLoginMiddleware, s.CheckCourseAdmin(q), s.EnsureCourseAdmin)

				// Get course admins (course admin)
				r.Method("GET", "/", s.GetCourseAdmins(q))

				// Add course admins (course admin)
				r.Method("POST", "/", s.AddCourseAdmins(q))

				// Overwrite course admins (course admin)
				r.Method("PUT", "/", s.UpdateCourseAdmins(q))

				// Remove course admins (course admin)
				r.Method("DELETE", "/", s.RemoveCourseAdmins(q))
			})
		})
	})

	// Queue by ID endpoints
	s.Route("/queues/{id:[a-zA-Z0-9]{27}}", func(r chi.Router) {
		r.Use(s.QueueIDMiddleware(q), s.CheckCourseAdmin(q))

		// Get queue by ID (more information with queue admin)
		r.Method("GET", "/", s.GetQueue(q))

		r.Method("GET", "/ws", s.QueueWebsocket())

		r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("PUT", "/", s.UpdateQueue(q))

		r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("DELETE", "/", s.RemoveQueue(q))

		// Get queue's stack (queue admin)
		r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("GET", "/stack", s.GetQueueStack(q))

		// Entry by ID endpoints
		r.Route("/entries", func(r chi.Router) {
			r.Use(s.ValidLoginMiddleware)

			// Add queue entry (valid login)
			// Rate limited to 30 requests per 15 minutes for a user to prevent abuse.
			r.With(s.rateLimiter(30, 15*time.Minute)).Method("POST", "/", s.AddQueueEntry(q))

			// Update queue entry (valid login, same user as creator)
			r.Method("PUT", "/{entry_id:[a-zA-Z0-9]{27}}", s.UpdateQueueEntry(q))

			// Remove queue entry (valid login, same user or queue admin)
			r.Method("DELETE", "/{entry_id:[a-zA-Z0-9]{27}}", s.RemoveQueueEntry(q))

			// Pin queue entry (course admin)
			r.With(s.EnsureCourseAdmin).Method("POST", "/{entry_id:[a-zA-Z0-9]{27}}/pin", s.PinQueueEntry(q))

			// Set queue entry helped state (course admin)
			r.With(s.EnsureCourseAdmin).Method("PUT", "/{entry_id:[a-zA-Z0-9]{27}}/helping", s.SetQueueEntryHelping(q))

			// Set student not helped (queue admin)
			r.With(s.EnsureCourseAdmin).Method("DELETE", "/{entry_id:[a-zA-Z0-9]{27}}/helped", s.SetNotHelped(q))

			// Randomize queue (course admin)
			r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("POST", "/randomize", s.RandomizeQueueEntries(q))

			// Clear queue (queue admin)
			r.With(s.EnsureCourseAdmin).Method("DELETE", "/", s.ClearQueueEntries(q))
		})

		// Announcements endpoints
		r.Route("/announcements", func(r chi.Router) {
			r.Use(s.ValidLoginMiddleware, s.EnsureCourseAdmin)

			// Create announcement (queue admin)
			r.Method("POST", "/", s.AddQueueAnnouncement(q))

			// Remove announcement (queue admin)
			r.Method("DELETE", "/{announcement_id:[a-zA-Z0-9]{27}}", s.RemoveQueueAnnouncement(q))
		})

		// Queue-wide (all days) schedule endpoints
		r.Route("/schedule", func(r chi.Router) {
			// Get queue schedule
			r.Method("GET", "/", s.GetQueueSchedule(q))

			// Update queue schedule (queue admin)
			r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("PUT", "/", s.UpdateQueueSchedule(q))
		})

		// Queue configuration endpoints
		r.Route("/configuration", func(r chi.Router) {
			// Get queue configuration
			r.Method("GET", "/", s.GetQueueConfiguration(q))

			// Update queue configuration (queue admin)
			r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("PUT", "/", s.UpdateQueueConfiguration(q))

			// Set manual queue open status (queue admin)
			r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("PUT", "/manual-open", s.UpdateQueueOpenStatus(q))
		})

		// Send message (queue admin)
		r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("POST", "/messages", s.SendMessage())

		// Get queue roster (queue admin)
		r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("GET", "/roster", s.GetQueueRoster(q))

		// Queue groups endpoints
		r.Route("/groups", func(r chi.Router) {
			r.Use(s.ValidLoginMiddleware, s.EnsureCourseAdmin)

			// Get queue groups (queue admin)
			r.Method("GET", "/", s.GetQueueGroups(q))

			// Update queue groups (queue admin)
			r.Method("PUT", "/", s.UpdateQueueGroups(q))
		})

		// Appointments endpoints
		r.Route("/appointments", func(r chi.Router) {
			// Specific day endpoints
			r.Route(`/{day:\d+}`, func(r chi.Router) {
				r.Use(s.AppointmentDayMiddleware)

				// Get endpoints on day (more information with queue admin)
				r.Method("GET", "/", s.GetAppointments(q))

				// Get appointments for current user on day
				r.With(s.ValidLoginMiddleware).Method("GET", "/@me", s.GetAppointmentsForCurrentUser(q))

				// Create appointment on day at timeslot
				r.With(s.ValidLoginMiddleware, s.rateLimiter(30, 15*time.Minute), s.AppointmentTimeslotMiddleware).Method("POST", `/{timeslot:\d+}`, s.SignupForAppointment(q))

				// Appointment claiming (queue admin)
				r.Route(`/claims/{timeslot:\d+}`, func(r chi.Router) {
					r.Use(s.ValidLoginMiddleware, s.EnsureCourseAdmin, s.AppointmentTimeslotMiddleware)

					// Claim appointment on day at timeslot (queue admin)
					r.Method("PUT", "/", s.ClaimTimeslot(q))
				})
			})

			// Existing appointment claims by ID (queue admin)
			r.Route(`/claims/{appointment_id:[a-zA-Z0-9]{27}}`, func(r chi.Router) {
				r.Use(s.ValidLoginMiddleware, s.EnsureCourseAdmin, s.AppointmentIDMiddleware(q))

				// Un-claim appointment (queue admin)
				r.Method("DELETE", "/", s.UnclaimAppointment(q))
			})

			// Appointment by ID endpoints
			r.Route(`/{appointment_id:[a-zA-Z0-9]{27}}`, func(r chi.Router) {
				r.Use(s.ValidLoginMiddleware, s.AppointmentIDMiddleware(q))

				// Update appointment (valid login, same user as creator)
				r.Method("PUT", "/", s.UpdateAppointment(q))

				// Cancel appointment (valid login, same user as creator)
				r.Method("DELETE", "/", s.RemoveAppointmentSignup(q))
			})

			// Appointment schedule endpoints
			r.Route("/schedule", func(r chi.Router) {
				// Get appointment schedule for all days
				r.Method("GET", "/", s.GetAppointmentSchedule(q))

				// Per-day schedules
				r.Route(`/{day:\d+}`, func(r chi.Router) {
					r.Use(s.AppointmentDayMiddleware)

					// Get appointment schedule for day
					r.Method("GET", "/", s.GetAppointmentScheduleForDay(q))

					// Update appointment schedule for day (queue admin)
					r.With(s.ValidLoginMiddleware, s.EnsureCourseAdmin).Method("PUT", "/", s.UpdateAppointmentSchedule(q))
				})
			})
		})
	})

	s.Method("GET", "/oauth2login", s.OAuth2LoginLink())

	// To not overwhelm our IdP with requests...
	s.With(s.rateLimiter(15, 15*time.Minute)).Method("GET", "/oauth2callback", s.OAuth2Callback())

	s.Method("GET", "/logout", s.Logout())

	s.With(s.ValidLoginMiddleware, s.EnsureSiteAdmin(q, false)).Method("GET", "/users/@am-site-admin", s.FowardAuth())

	s.With(s.ValidLoginMiddleware).Method("GET", "/users/@me", s.GetCurrentUserInfo(q))

	s.Method("GET", "/metrics", s.MetricsHandler())

	s.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	s.RegisterQueueStats(q)

	return &s
}
