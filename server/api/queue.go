package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/ksuid"
)

func init() {
	prometheus.MustRegister(websocketCounter, websocketEventCounter)
}

type getQueue interface {
	GetQueue(context.Context, ksuid.KSUID) (*Queue, error)
}

const queueContextKey = "queue"

// Maximum character limits for queue entry fields
const maxDescriptionLength = 1500
const maxLocationLength = 300

func (s *Server) QueueIDMiddleware(gq getQueue) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := s.getCtxLogger(r)

			idString := chi.URLParam(r, "id")
			id, err := ksuid.Parse(idString)
			if err != nil {
				l.Warnw("failed to parse queue id", "queue_id", idString)
				s.errorMessage(
					http.StatusNotFound,
					"That queue is hiding from me…make sure it exists!",
					w, r,
				)
				return
			}

			q, err := gq.GetQueue(r.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				l.Warnw("failed to get non-existent queue with valid ksuid", "queue_id", idString)
				s.errorMessage(
					http.StatusNotFound,
					"That queue is hiding from me…make sure it exists!",
					w, r,
				)
				return
			} else if err != nil {
				l.Errorw("failed to get queue", "queue_id", idString, "err", err)
				s.internalServerError(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), queueContextKey, q)
			ctx = context.WithValue(ctx, loggerContextKey, l.With(
				"queue_id", q.ID,
				"course_id", q.Course,
			))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type getQueueEntry interface {
	GetQueueEntry(ctx context.Context, entry ksuid.KSUID, allowRemoved bool) (*QueueEntry, error)
}

type getQueueEntries interface {
	GetQueueEntries(ctx context.Context, queue ksuid.KSUID, admin bool) ([]*QueueEntry, error)
}

type getActiveQueueEntriesForUser interface {
	GetActiveQueueEntriesForUser(ctx context.Context, queue ksuid.KSUID, email string) ([]*QueueEntry, error)
}

type getQueueAnnouncements interface {
	GetQueueAnnouncements(ctx context.Context, queue ksuid.KSUID) ([]*Announcement, error)
}

type getQueueStack interface {
	GetQueueStack(ctx context.Context, queue ksuid.KSUID, limit int) ([]*RemovedQueueEntry, error)
}

type getCurrentDaySchedule interface {
	GetCurrentDaySchedule(ctx context.Context, queue ksuid.KSUID) (string, error)
}

type getQueueDetails interface {
	getQueueEntry
	getQueueEntries
	getActiveQueueEntriesForUser
	getQueueStack
	getQueueAnnouncements
	getCurrentDaySchedule
	getQueueConfiguration
}

func (s *Server) GetQueue(gd getQueueDetails) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		l := s.getCtxLogger(r)

		admin := r.Context().Value(courseAdminContextKey).(bool)
		// This is a bit of a hack, but we're okay with the zero value
		// of string if the assertion fails, but we don't want it to panic,
		// so we need to do the two-value assertion
		email, _ := r.Context().Value(emailContextKey).(string)

		// This isn't pretty, but it resembles the dynamic
		// response structure of the PHP API
		response := map[string]interface{}{}
		entries, err := gd.GetQueueEntries(r.Context(), q.ID, admin)
		if err != nil {
			l.Errorw("failed to get queue entries", "err", err)
			return err
		}

		// If user is logged in but not admin, check to
		// add their info to their queue entry(-ies)
		if !admin && email != "" {
			userEntries, err := gd.GetActiveQueueEntriesForUser(r.Context(), q.ID, email)
			if err != nil {
				l.Errorw("failed to get active queue entries for user",
					"err", err,
				)
				return err
			}

			for _, userEntry := range userEntries {
				for i, e := range entries {
					if userEntry.ID == e.ID {
						entries[i] = userEntry
						break
					}
				}
			}
		}
		response["queue"] = entries

		if admin {
			stack, err := gd.GetQueueStack(r.Context(), q.ID, 20)
			if err != nil {
				l.Errorw("failed to get queue stack", "err", err)
				return err
			}
			response["stack"] = stack

			s.websocketCountLock.Lock()
			m := make([]string, 0, len(s.websocketCountByEmail[q.ID]))
			for e := range s.websocketCountByEmail[q.ID] {
				m = append(m, e)
			}
			s.websocketCountLock.Unlock()
			response["online"] = m
		}

		config, err := gd.GetQueueConfiguration(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to get queue configuration", "err", err)
			return err
		}
		response["config"] = config

		schedule, err := gd.GetCurrentDaySchedule(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to get queue schedule", "err", err)
			return err
		}
		response["schedule"] = schedule

		halfHour := CurrentHalfHour()
		response["half_hour"] = halfHour
		if config.Scheduled {
			response["open"] = schedule[halfHour] == 'o' || schedule[halfHour] == 'p'
		} else {
			response["open"] = config.ManualOpen
		}

		announcements, err := gd.GetQueueAnnouncements(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to get queue announcements", "err", err)
			return err
		}
		response["announcements"] = announcements

		return s.sendResponse(http.StatusOK, response, w, r)
	}
}

var websocketCounter = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "websocket_count",
		Help: "The number of connected WebSocket clients per queue.",
	},
	[]string{"queue"},
)

var websocketEventCounter = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "websocket_event_count",
		Help: "The number and type of WebSocket events sent (in total, to all clients) per queue.",
	},
	[]string{"queue", "event"},
)

var upgrader = &websocket.Upgrader{
	HandshakeTimeout: 30 * time.Second,
}

func (s *Server) QueueWebsocket() E {
	type update struct {
		Email  string `json:"email"`
		Status string `json:"status"`
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		var topics []string

		q := r.Context().Value(queueContextKey).(*Queue)
		topics = append(topics, QueueTopicGeneric(q.ID))

		admin := r.Context().Value(courseAdminContextKey).(bool)
		if admin {
			topics = append(topics, QueueTopicAdmin(q.ID))
		} else {
			topics = append(topics, QueueTopicNonPrivileged(q.ID))
		}

		// Yes, this is okay---see above
		email, _ := r.Context().Value(emailContextKey).(string)
		if email != "" {
			topics = append(topics, QueueTopicEmail(q.ID, email))
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to upgrade to websocket connection", "err", err)
			return StatusError{
				status: http.StatusBadRequest,
			}
		}

		events := s.ps.Sub(topics...)

		s.websocketCountLock.Lock()

		ws := s.websocketCount[q.ID]
		ws++
		s.websocketCount[q.ID] = ws

		websocketCounter.With(prometheus.Labels{"queue": q.ID.String()}).Set(float64(ws))

		first := false
		if email != "" {
			e := s.websocketCountByEmail[q.ID]
			if e == nil {
				e = make(map[string]int)
				s.websocketCountByEmail[q.ID] = e
			}
			first = e[email] == 0
			e[email]++
		}

		s.websocketCountLock.Unlock()

		s.ps.Pub(WS("QUEUE_CONNECTIONS_UPDATE", ws), QueueTopicAdmin(q.ID))
		if first {
			s.ps.Pub(WS("USER_STATUS_UPDATE", update{Email: email, Status: "online"}), QueueTopicAdmin(q.ID))
		}

		if email != "" {
			s.getCtxLogger(r).Info("websocket connection opened")
		}

		// The interval at which the server will expect pings from the client.
		const pingInterval = 10 * time.Second

		// The "slack" built into the ping logic; the extra time allowed
		// to clients to ping past the interval.
		const pingSlack = 2 * time.Second

		go func() {
			for {
				conn.SetReadDeadline(time.Now().Add(pingInterval + pingSlack))
				_, _, err := conn.ReadMessage()
				if err != nil {
					s.ps.Unsub(events)
					conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
						time.Now().Add(pingSlack),
					)
					conn.Close()

					s.websocketCountLock.Lock()

					ws := s.websocketCount[q.ID]
					ws--
					s.websocketCount[q.ID] = ws

					websocketCounter.With(prometheus.Labels{"queue": q.ID.String()}).Set(float64(s.websocketCount[q.ID]))

					last := false
					if email != "" {
						e := s.websocketCountByEmail[q.ID]
						last = e[email] == 1
						e[email]--
						if last {
							delete(e, email)
						}
					}

					s.websocketCountLock.Unlock()

					s.ps.Pub(WS("QUEUE_CONNECTIONS_UPDATE", ws), QueueTopicAdmin(q.ID))
					if last {
						s.ps.Pub(WS("USER_STATUS_UPDATE", update{Email: email, Status: "offline"}), QueueTopicAdmin(q.ID))
					}

					if email != "" {
						s.getCtxLogger(r).Info("websocket connection closed")
					}
					return
				}
			}
		}()

		go func() {
			pingTicker := time.NewTicker(pingInterval)
			defer pingTicker.Stop()
			for {
				var eventName string
				select {
				case <-pingTicker.C:
					// Using a custom ping message rather than a ping control
					// frame because browsers can't access control frames :(
					err = conn.WriteJSON(WS("PING", nil))
					eventName = "PING"
				case event, ok := <-events:
					if !ok {
						return
					}
					err = conn.WriteJSON(event)
					e, ok := event.(*WSMessage)
					if ok {
						eventName = e.Event
					}
				}
				websocketEventCounter.With(prometheus.Labels{"queue": q.ID.String(), "event": eventName}).Inc()

				// If the write fails, we presume that the read will also
				// fail, so the read loop will take care of unsubbing and
				// closing the connection. We also can't unsub on the same
				// goroutine from which we're listening for events. We should
				// just return.
				if err != nil {
					return
				}
			}
		}()

		return nil
	}
}

type updateQueue interface {
	UpdateQueue(ctx context.Context, queue ksuid.KSUID, values *Queue) error
}

func (s *Server) UpdateQueue(uq updateQueue) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		l := s.getCtxLogger(r)

		var queue Queue
		err := json.NewDecoder(r.Body).Decode(&queue)
		if err != nil {
			l.Warnw("failed to decode queue from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the queue from the request body.",
			}
		}

		if queue.Name == "" {
			l.Warnw("got incomplete queue", "queue", queue)
			return StatusError{
				http.StatusBadRequest,
				"It looks like you missed some fields in the queue!",
			}
		}

		err = uq.UpdateQueue(r.Context(), q.ID, &queue)
		if err != nil {
			l.Errorw("failed to update queue", "err", err)
			return err
		}

		l.Infow("updated queue")
		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type removeQueue interface {
	RemoveQueue(ctx context.Context, queue ksuid.KSUID) error
}

func (s *Server) RemoveQueue(rq removeQueue) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		l := s.getCtxLogger(r)

		err := rq.RemoveQueue(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to remove queue", "err", err)
			return err
		}

		l.Infow("removed queue")
		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

func (s *Server) GetQueueStack(gs getQueueStack) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		stack, err := gs.GetQueueStack(r.Context(), q.ID, 10000)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to fetch stack",
				"err", err,
			)
			return err
		}

		s.getCtxLogger(r).Infow("fetched stack",
			"stack_length", len(stack),
		)
		return s.sendResponse(http.StatusOK, stack, w, r)
	}
}

type canAddEntry interface {
	CanAddEntry(ctx context.Context, queue ksuid.KSUID, email string) (bool, error)
}

type addQueueEntry interface {
	getQueueEntries
	getActiveQueueEntriesForUser
	canAddEntry
	GetEntryPriority(ctx context.Context, queue ksuid.KSUID, email string) (int, error)
	AddQueueEntry(context.Context, *QueueEntry) (*QueueEntry, error)
	getQueueConfiguration
}

// validateQueueEntryDescription validates that:
// - If prompts are configured, description must be valid JSON array matching prompt count
// - If no prompts configured, description must not be JSON
// - Description must not exceed the maximum character limit
func validateQueueEntryDescription(description string, prompts []string) error {
	// Check length first
	if len(description) > maxDescriptionLength {
		return fmt.Errorf("description is too long (max %d characters)", maxDescriptionLength)
	}

	var jsonArray []string
	err := json.Unmarshal([]byte(description), &jsonArray)

	// If prompts are configured, description should be a JSON array
	if len(prompts) > 0 {
		if err != nil {
			return fmt.Errorf("hmm, got description in unexpected format. Try clear cache and refresh?")
		}

		// Check that the array length matches the number of prompts
		if len(jsonArray) != len(prompts) {
			return fmt.Errorf("wrong number of prompt responses, expected %d got %d", len(prompts), len(jsonArray))
		}

		// Verify all responses are non-empty
		for i, response := range jsonArray {
			if strings.TrimSpace(response) == "" {
				return fmt.Errorf("empty response for prompt #%d: %s", i+1, prompts[i])
			}
		}

		return nil
	}

	// If no prompts configured, check if description is accidentally JSON array
	if err := json.Unmarshal([]byte(description), &jsonArray); err == nil {
		return fmt.Errorf("oops, JSON array-like string is not allowed as description")
	}

	// Should also not be dictionary-like
	if err := json.Unmarshal([]byte(description), &map[string]interface{}{}); err == nil {
		return fmt.Errorf("oops, JSON object-like string is not allowed as description")
	}

	return nil
}

func (s *Server) AddQueueEntry(ae addQueueEntry) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		email := r.Context().Value(emailContextKey).(string)
		name := r.Context().Value(nameContextKey).(string)
		l := s.getCtxLogger(r)

		currentEntries, err := ae.GetActiveQueueEntriesForUser(r.Context(), q.ID, email)
		if err != nil {
			l.Errorw("failed to fetch current queue entries for user", "err", err)
			return err
		}

		if len(currentEntries) > 0 {
			l.Warnw("attempted queue sign up with already existing entry",
				"conflicting_entry", currentEntries[0].ID,
			)
			return StatusError{
				http.StatusConflict,
				"Don't get greedy! You can only be on the queue once at a time.",
			}
		}

		canSignUp, err := ae.CanAddEntry(r.Context(), q.ID, email)
		if err != nil || !canSignUp {
			l.Warnw("user attempting to sign up for queue not allowed to", "err", err, "user-agent", r.UserAgent())
			return StatusError{
				http.StatusForbidden,
				"My records say you aren't allowed to sign up right now: " + err.Error() + ".",
			}
		}

		var entry QueueEntry
		err = json.NewDecoder(r.Body).Decode(&entry)
		if err != nil {
			l.Warnw("failed to decode queue entry from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the queue entry from the request body.",
			}
		}

		entry.Queue = q.ID
		entry.Email = email
		entry.Name = name
		// Don't check location because it could be a map location;
		// we're using the frontend as a bit of a crutch here
		if entry.Description == "" || entry.Name == "" {
			l.Warnw("incomplete queue entry", "entry", entry)
			return StatusError{
				http.StatusBadRequest,
				"It looks like you left out some fields in the queue entry!",
			}
		}

		if len(entry.Location) > maxLocationLength {
			l.Warnw("location too long", "location_length", len(entry.Location))
			return StatusError{
				http.StatusBadRequest,
				fmt.Sprintf("Location field is too long (max %d characters)", maxLocationLength),
			}
		}

		// Validate description format if prompts are configured
		config, err := ae.GetQueueConfiguration(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to get queue configuration", "err", err)
			return err
		}

		var prompts []string
		if err := json.Unmarshal(config.Prompts, &prompts); err != nil {
			l.Errorw("failed to unmarshal prompts", "err", err)
			return err
		}

		if err := validateQueueEntryDescription(entry.Description, prompts); err != nil {
			l.Warnw("invalid entry description", "err", err)
			return StatusError{
				http.StatusBadRequest,
				err.Error(),
			}
		}

		priority, err := ae.GetEntryPriority(r.Context(), q.ID, email)
		if err != nil {
			l.Errorw("failed to get entry priority", "err", err)
			return err
		}
		entry.Priority = priority

		newEntry, err := ae.AddQueueEntry(r.Context(), &entry)
		if err != nil {
			var p *pq.Error
			if errors.As(err, &p) {
				l.Warnw("attempted queue sign up with already existing entry", "err", err)
				return StatusError{
					http.StatusConflict,
					"Don't get greedy! You can only be on the queue once at a time.",
				}
			}
			l.Errorw("failed to insert queue entry", "err", err)
			return err
		}

		l.Infow("created queue entry", "entry_id", newEntry.ID)

		s.ps.Pub(WS("ENTRY_CREATE", newEntry), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_CREATE", newEntry.Anonymized()), QueueTopicNonPrivileged(q.ID))

		// Send an update with more information to the user who
		// created the queue entry.
		s.ps.Pub(WS("ENTRY_UPDATE", newEntry), QueueTopicEmail(q.ID, email))

		return s.sendResponse(http.StatusCreated, newEntry, w, r)
	}
}

type updateQueueEntry interface {
	getQueueEntry
	UpdateQueueEntry(ctx context.Context, entry ksuid.KSUID, newEntry *QueueEntry) error
	getQueueConfiguration
}

func (s *Server) UpdateQueueEntry(ue updateQueueEntry) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		id := chi.URLParam(r, "entry_id")
		email := r.Context().Value(emailContextKey).(string)
		name := r.Context().Value(nameContextKey).(string)
		l := s.getCtxLogger(r).With("entry_id", id)

		entry, err := ksuid.Parse(id)
		if err != nil {
			l.Warnw("failed to parse entry ID", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		e, err := ue.GetQueueEntry(r.Context(), entry, false)
		if err != nil {
			l.Warnw("failed to get entry with valid ksuid", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry. Perhaps you were popped off quite recently?",
			}
		}

		if e.Email != email {
			l.Warnw("user tried to update other user's queue entry", "entry_email", e.Email)
			return StatusError{
				http.StatusForbidden,
				"You can't edit someone else's queue entry!",
			}
		}

		var newEntry QueueEntry
		err = json.NewDecoder(r.Body).Decode(&newEntry)
		if err != nil {
			l.Warnw("failed to decode queue entry from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the queue entry from the request body.",
			}
		}
		newEntry.Name = name

		if newEntry.Name == "" || newEntry.Description == "" {
			l.Warnw("incomplete queue entry", "entry", entry)
			return StatusError{
				http.StatusBadRequest,
				"It looks like you left out some fields in the queue entry!",
			}
		}

		// Check location length
		if len(newEntry.Location) > maxLocationLength {
			l.Warnw("location too long", "location_length", len(newEntry.Location))
			return StatusError{
				http.StatusBadRequest,
				fmt.Sprintf("Location field is too long (max %d characters)", maxLocationLength),
			}
		}

		// Validate description format if prompts are configured
		config, err := ue.GetQueueConfiguration(r.Context(), q.ID)
		if err != nil {
			l.Errorw("failed to get queue configuration", "err", err)
			return err
		}

		var prompts []string
		if err := json.Unmarshal(config.Prompts, &prompts); err != nil {
			l.Errorw("failed to unmarshal prompts", "err", err)
			return err
		}

		if err := validateQueueEntryDescription(newEntry.Description, prompts); err != nil {
			l.Warnw("invalid entry description", "err", err)
			return StatusError{
				http.StatusBadRequest,
				err.Error(),
			}
		}

		err = ue.UpdateQueueEntry(r.Context(), entry, &newEntry)
		if err != nil {
			l.Errorw("failed to update queue entry", "err", err)
			return err
		}

		l.Infow("queue entry updated", "old_entry", e)

		newEntry.ID = entry
		newEntry.Queue = q.ID
		newEntry.Email = e.Email
		newEntry.Pinned = e.Pinned
		newEntry.Helping = e.Helping
		newEntry.Priority = e.Priority

		s.ps.Pub(WS("ENTRY_UPDATE", &newEntry), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_UPDATE", &newEntry), QueueTopicEmail(q.ID, email))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type canRemoveQueueEntry interface {
	CanRemoveQueueEntry(ctx context.Context, queue ksuid.KSUID, entry ksuid.KSUID, email string) (bool, error)
}

type removeQueueEntry interface {
	canRemoveQueueEntry
	RemoveQueueEntry(ctx context.Context, entry ksuid.KSUID, remover string) (*RemovedQueueEntry, error)
}

func (s *Server) RemoveQueueEntry(re removeQueueEntry) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		id := chi.URLParam(r, "entry_id")
		email := r.Context().Value(emailContextKey).(string)
		l := s.getCtxLogger(r).With("entry_id", id)

		entry, err := ksuid.Parse(id)
		if err != nil {
			l.Warnw("failed to parse entry ID", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		canRemove, err := re.CanRemoveQueueEntry(r.Context(), q.ID, entry, email)
		if err != nil || !canRemove {
			l.Warnw("attempted to remove queue entry without access", "err", err)
			return StatusError{
				http.StatusForbidden,
				"Removing someone else's queue entry isn't very nice!",
			}
		}

		e, err := re.RemoveQueueEntry(r.Context(), entry, email)
		if errors.Is(err, sql.ErrNoRows) {
			l.Warnw("attempted to remove already-removed queue entry", "err", err)
			return StatusError{
				http.StatusNotFound,
				"That queue entry was already removed by another staff member! Try the next one on the queue.",
			}
		} else if err != nil {
			l.Errorw("failed to remove queue entry", "err", err)
			return err
		}

		l.Infow("removed queue entry",
			"student_email", e.Email,
			"time_spent", time.Now().Sub(e.ID.Time()),
		)

		s.ps.Pub(WS("ENTRY_REMOVE", e), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_REMOVE", e.Anonymized()), QueueTopicNonPrivileged(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type pinQueueEntry interface {
	getQueueEntry
	getActiveQueueEntriesForUser
	PinQueueEntry(ctx context.Context, entry ksuid.KSUID) error
}

func (s *Server) PinQueueEntry(pb pinQueueEntry) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		id := chi.URLParam(r, "entry_id")
		email := r.Context().Value(emailContextKey).(string)
		l := s.getCtxLogger(r).With("entry_id", id)

		entryID, err := ksuid.Parse(id)
		if err != nil {
			l.Warnw("failed to parse entry ID", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		entry, err := pb.GetQueueEntry(r.Context(), entryID, true)
		if err != nil {
			l.Warnw("attempted to get non-existent queue entry with valid ksuid")
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		entries, err := pb.GetActiveQueueEntriesForUser(r.Context(), q.ID, entry.Email)
		if err != nil {
			l.Errorw("failed to get queue entries for user")
			return err
		}

		if !entry.Active.Valid && len(entries) > 0 {
			l.Warnw("attempted to pin queue entry with student on queue")
			return StatusError{
				http.StatusConflict,
				"That user is already on the queue. Pin their new entry!",
			}
		}

		err = pb.PinQueueEntry(r.Context(), entryID)
		if err != nil {
			l.Errorw("failed to pin queue entry", "err", err)
			return err
		}

		entry.Pinned = true

		l.Infow("pinned queue entry")

		s.ps.Pub(WS("STACK_REMOVE", entry), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_CREATE", entry), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_CREATE", entry.Anonymized()), QueueTopicNonPrivileged(q.ID))

		// Send an update with more information to the user who
		// created the queue entry.
		s.ps.Pub(WS("ENTRY_UPDATE", entry), QueueTopicEmail(q.ID, email))
		s.ps.Pub(WS("ENTRY_PINNED", entry), QueueTopicEmail(q.ID, entry.Email))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type setQueueEntryHelping interface {
	getQueueEntry
	SetQueueEntryHelping(ctx context.Context, entry ksuid.KSUID, helping string) error
}

func (s *Server) SetQueueEntryHelping(eh setQueueEntryHelping) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		id := chi.URLParam(r, "entry_id")
		l := s.getCtxLogger(r).With("entry_id", id)

		var helping bool
		switch r.URL.Query().Get("helping") {
		case "true":
			helping = true
		case "false":
			helping = false // not technically necessary but meh
		default:
			l.Warnw("unknown helping value", "helping", r.URL.Query().Get("helping"))
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the helping status from the `helping` query parameter.",
			}
		}

		entryID, err := ksuid.Parse(id)
		if err != nil {
			l.Warnw("failed to parse entry ID", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		entry, err := eh.GetQueueEntry(r.Context(), entryID, true)
		if err != nil {
			l.Warnw("attempted to get non-existent queue entry with valid ksuid")
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		beingHelpedBy := ""
		if helping {
			beingHelpedBy = " " + r.Context().Value(firstNameContextKey).(string)
		}

		err = eh.SetQueueEntryHelping(r.Context(), entryID, beingHelpedBy)
		if err != nil {
			l.Errorw("failed to set helping status", "err", err)
			return err
		}

		entry.Helping = beingHelpedBy

		l.Infow("set helping status", "helping", helping)

		s.ps.Pub(WS("ENTRY_UPDATE", entry.Anonymized()), QueueTopicNonPrivileged(q.ID))
		s.ps.Pub(WS("ENTRY_UPDATE", entry), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("ENTRY_UPDATE", entry), QueueTopicEmail(q.ID, entry.Email))
		s.ps.Pub(WS("ENTRY_HELPING", entry), QueueTopicEmail(q.ID, entry.Email))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type randomizeQueueEntries interface {
	getQueueEntries
	RandomizeQueueEntries(ctx context.Context, queue ksuid.KSUID) error
}

func (s *Server) RandomizeQueueEntries(re randomizeQueueEntries) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		err := re.RandomizeQueueEntries(r.Context(), q.ID)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to randomize queue",
				"err", err,
			)
			return err
		}
		entries, err := re.GetQueueEntries(r.Context(), q.ID, true)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to get queue entries after randomization",
				"err", err,
			)
			return err
		}

		s.ps.Pub(WS("QUEUE_RANDOMIZE", nil), QueueTopicGeneric(q.ID))

		for _, e := range entries {
			s.ps.Pub(WS("ENTRY_UPDATE", e), QueueTopicAdmin(q.ID))
			s.ps.Pub(WS("ENTRY_UPDATE", e.Anonymized()), QueueTopicNonPrivileged(q.ID))
		}

		s.getCtxLogger(r).Info("randomized queue")

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type clearQueueEntries interface {
	ClearQueueEntries(ctx context.Context, queue ksuid.KSUID, remover string) error
}

func (s *Server) ClearQueueEntries(ce clearQueueEntries) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		email := r.Context().Value(emailContextKey).(string)
		err := ce.ClearQueueEntries(r.Context(), q.ID, email)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to clear queue", "err", err)
			return err
		}

		s.getCtxLogger(r).Info("cleared queue")

		s.ps.Pub(WS("QUEUE_CLEAR", email), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("QUEUE_CLEAR", nil), QueueTopicNonPrivileged(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type addQueueAnnouncement interface {
	AddQueueAnnouncement(context.Context, ksuid.KSUID, *Announcement) (*Announcement, error)
}

func (s *Server) AddQueueAnnouncement(aa addQueueAnnouncement) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		var announcement Announcement
		err := json.NewDecoder(r.Body).Decode(&announcement)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to decode announcement from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the announcement from the request body.",
			}
		}

		announcement.Queue = q.ID
		if announcement.Content == "" {
			s.getCtxLogger(r).Warnw("received incomplete announcement", "announcement", announcement)
			return StatusError{
				http.StatusBadRequest,
				"It looks like you left out some fields in the announcement.",
			}
		}

		newAnnouncement, err := aa.AddQueueAnnouncement(r.Context(), q.ID, &announcement)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to create new announcement",
				"announcement", announcement,
				"err", err,
			)
			return err
		}

		s.getCtxLogger(r).Infow("created announcement",
			"announcement", newAnnouncement,
		)

		s.ps.Pub(WS("ANNOUNCEMENT_CREATE", newAnnouncement), QueueTopicGeneric(q.ID))

		return s.sendResponse(http.StatusCreated, newAnnouncement, w, r)
	}
}

type removeQueueAnnouncement interface {
	RemoveQueueAnnouncement(context.Context, ksuid.KSUID) error
}

func (s *Server) RemoveQueueAnnouncement(ra removeQueueAnnouncement) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		id := chi.URLParam(r, "announcement_id")
		announcement, err := ksuid.Parse(id)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to parse announcement ID",
				"announcement_id", id,
				"err", err,
			)
			return StatusError{
				http.StatusNotFound,
				"I couldn't find that announcement anywhere.",
			}
		}

		err = ra.RemoveQueueAnnouncement(r.Context(), announcement)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to remove announcement",
				"announcement_id", announcement,
				"err", err,
			)
			return err
		}

		s.getCtxLogger(r).Infow("removed announcement",
			"announcement_id", announcement,
		)

		s.ps.Pub(WS("ANNOUNCEMENT_DELETE", announcement.String()), QueueTopicGeneric(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type getQueueSchedule interface {
	GetQueueSchedule(ctx context.Context, queue ksuid.KSUID) ([]string, error)
}

func (s *Server) GetQueueSchedule(gs getQueueSchedule) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		schedules, err := gs.GetQueueSchedule(r.Context(), q.ID)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to get queue schedule", "err", err)
			return err
		}

		return s.sendResponse(http.StatusOK, schedules, w, r)
	}
}

type updateQueueSchedule interface {
	UpdateQueueSchedule(ctx context.Context, queue ksuid.KSUID, schedules []string) error
}

func (s *Server) UpdateQueueSchedule(us updateQueueSchedule) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		var schedules []string
		err := json.NewDecoder(r.Body).Decode(&schedules)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to decode schedules", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the schedules from the request body.",
			}
		}

		for i, schedule := range schedules {
			if len(schedule) != 48 {
				s.getCtxLogger(r).Warnw("got schedule with length not 48",
					"len", len(schedule),
					"day", i,
					"schedule", schedule,
				)
				return StatusError{
					http.StatusBadRequest,
					"Make sure your schedule is 48 characters long!",
				}
			}
		}

		err = us.UpdateQueueSchedule(r.Context(), q.ID, schedules)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to update schedule", "err", err)
			return err
		}

		s.getCtxLogger(r).Infow("updated queue schedule", "schedules", schedules)

		s.ps.Pub(WS("REFRESH", nil), QueueTopicGeneric(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type getQueueConfiguration interface {
	GetQueueConfiguration(ctx context.Context, queue ksuid.KSUID) (*QueueConfiguration, error)
}

func (s *Server) GetQueueConfiguration(gc getQueueConfiguration) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		config, err := gc.GetQueueConfiguration(r.Context(), q.ID)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to get queue configuration", "err", err)
			return err
		}

		return s.sendResponse(http.StatusOK, config, w, r)
	}
}

type updateQueueConfiguration interface {
	UpdateQueueConfiguration(ctx context.Context, queue ksuid.KSUID, configuration *QueueConfiguration) error
}

func (s *Server) UpdateQueueConfiguration(uc updateQueueConfiguration) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		var config QueueConfiguration
		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to decode configuration", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the configuration from the request body.",
			}
		}

		// Validate prompt format
		var prompts []string
		if err := json.Unmarshal(config.Prompts, &prompts); err != nil {
			s.getCtxLogger(r).Warnw("failed to unmarshal prompts", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"Invalid customized prompts format.",
			}
		}

		// Check no duplicate prompts by compare length of prompts and set
		promptSet := make(map[string]struct{})
		for _, prompt := range prompts {
			promptSet[prompt] = struct{}{}
		}
		if len(prompts) != len(promptSet) {
			s.getCtxLogger(r).Warnw("duplicate prompts", "prompts", prompts)
			return StatusError{
				http.StatusBadRequest,
				"Customized prompts contain duplicates.",
			}
		}

		err = uc.UpdateQueueConfiguration(r.Context(), q.ID, &config)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to update queue configuration", "err", err)
			return err
		}

		s.getCtxLogger(r).Infow("updated queue configuration", "configuration", config)

		s.ps.Pub(WS("REFRESH", nil), QueueTopicGeneric(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type updateQueueOpenStatus interface {
	UpdateQueueOpenStatus(ctx context.Context, queue ksuid.KSUID, open bool) error
}

func (s *Server) UpdateQueueOpenStatus(uo updateQueueOpenStatus) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		var open bool
		switch r.URL.Query().Get("open") {
		case "true":
			open = true
		case "false":
			open = false // not technically necessary but meh
		default:
			s.getCtxLogger(r).Warnw("unknown open query value", "open", open)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the open status from the `open` query parameter.",
			}
		}

		err := uo.UpdateQueueOpenStatus(r.Context(), q.ID, open)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to update queue open status", "err", err)
			return err
		}

		s.getCtxLogger(r).Infow("updated queue open status", "open", open)

		s.ps.Pub(WS("QUEUE_OPEN", open), QueueTopicGeneric(q.ID))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

func (s *Server) SendMessage() E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		l := s.getCtxLogger(r)

		var message Message
		err := json.NewDecoder(r.Body).Decode(&message)
		if err != nil {
			l.Warnw("failed to decode message from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				"We couldn't read the message from the request body.",
			}
		}

		if message.Receiver == "" || message.Content == "" {
			l.Warnw("got incomplete message", "message", message)
			return StatusError{
				http.StatusBadRequest,
				"It looks like you left out some fields from the message.",
			}
		}

		// Sender doesn't really matter as frontend is not showing it
		// Keep redacted for privacy
		message.Sender = ""
		message.Queue = q.ID
		message.ID = ksuid.New()

		if message.Receiver == "<broadcast>" {
			l.Infow("broadcast to queue", "content", message.Content)
			s.ps.Pub(WS("MESSAGE_CREATE", message), QueueTopicGeneric(q.ID))
		} else {
			l.Infow("send DM", "message", message, "to_user", message.Receiver)
			s.ps.Pub(WS("MESSAGE_CREATE", message), QueueTopicEmail(q.ID, message.Receiver))
		}

		return s.sendResponse(http.StatusCreated, message, w, r)
	}
}

type getQueueRoster interface {
	GetQueueRoster(ctx context.Context, queue ksuid.KSUID) ([]string, error)
}

func (s *Server) GetQueueRoster(gr getQueueRoster) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		roster, err := gr.GetQueueRoster(r.Context(), q.ID)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to fetch queue roster", "err", err)
			return err
		}

		return s.sendResponse(http.StatusOK, roster, w, r)
	}
}

type getQueueGroups interface {
	GetQueueGroups(ctx context.Context, queue ksuid.KSUID) ([][]string, error)
}

func (s *Server) GetQueueGroups(gg getQueueGroups) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		groups, err := gg.GetQueueGroups(r.Context(), q.ID)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to fetch queue groups", "err", err)
			return err
		}

		return s.sendResponse(http.StatusOK, groups, w, r)
	}
}

type updateQueueGroups interface {
	UpdateQueueRoster(ctx context.Context, queue ksuid.KSUID, students []string) error
	UpdateQueueGroups(ctx context.Context, queue ksuid.KSUID, groups [][]string) error
}

func (s *Server) UpdateQueueGroups(ug updateQueueGroups) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)

		var groups [][]string
		err := json.NewDecoder(r.Body).Decode(&groups)
		if err != nil {
			s.getCtxLogger(r).Warnw("failed to read groups from body", "err", err)
			return StatusError{
				http.StatusBadRequest,
				fmt.Sprintf("I couldn't read the groups you uploaded. Make sure the file is structured as an array of arrays of students' emails, each inner array representing a group. This error might help: %v", err),
			}
		}

		err = ug.UpdateQueueGroups(r.Context(), q.ID, groups)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to update groups", "err", err)
			return err
		}

		var students []string
		for _, group := range groups {
			for _, student := range group {
				students = append(students, student)
			}
		}

		err = ug.UpdateQueueRoster(r.Context(), q.ID, students)
		if err != nil {
			s.getCtxLogger(r).Errorw("failed to update roster", "err", err)
			return err
		}

		s.getCtxLogger(r).Infow("updated groups")
		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}

type setNotHelped interface {
	getQueueEntry
	SetHelpedStatus(ctx context.Context, entry ksuid.KSUID, helped bool) error
}

func (s *Server) SetNotHelped(sh setNotHelped) E {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.Context().Value(queueContextKey).(*Queue)
		id := chi.URLParam(r, "entry_id")
		l := s.getCtxLogger(r).With("entry_id", id)

		entryID, err := ksuid.Parse(id)
		if err != nil {
			l.Warnw("failed to parse entry ID", "err", err)
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		entry, err := sh.GetQueueEntry(r.Context(), entryID, true)
		if err != nil {
			l.Warnw("attempted to get non-existent queue entry with valid ksuid")
			return StatusError{
				http.StatusNotFound,
				"I'm not able to find that queue entry.",
			}
		}

		err = sh.SetHelpedStatus(r.Context(), entryID, false)
		if err != nil {
			l.Errorw("failed to set entry to not helped", "err", err)
			return err
		}

		entry.Helped = false

		l.Infow("set entry to not helped")

		s.ps.Pub(WS("ENTRY_UPDATE", entry.RemovedEntry()), QueueTopicAdmin(q.ID))
		s.ps.Pub(WS("NOT_HELPED", nil), QueueTopicEmail(q.ID, entry.Email))

		return s.sendResponse(http.StatusNoContent, nil, w, r)
	}
}
