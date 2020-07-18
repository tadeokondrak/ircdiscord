package server

import (
	"log"
	"net"
	"sync"

	"github.com/diamondburned/arikawa/discord"
	"github.com/pkg/errors"
	"github.com/tadeokondrak/ircdiscord/internal/client"
	"github.com/tadeokondrak/ircdiscord/internal/session"
)

// Server is the collection of state shared across all connections.
type Server struct {
	listener     net.Listener // never changes after construction
	IRCDebug     bool
	DiscordDebug bool

	mu       sync.Mutex                             // guards next 3 fields
	ids      map[string]discord.Snowflake           // tokens to IDs
	sessions map[discord.Snowflake]*session.Session // IDs to sessions
	clients  []*client.Client                       // active clients
}

// New creates a new Server, taking ownership of the listener.
func New(listener net.Listener) *Server {
	return &Server{
		listener: listener,
		ids:      make(map[string]discord.Snowflake),
		sessions: make(map[discord.Snowflake]*session.Session),
	}
}

// Close closes the server's listener, which causes the current Run invocation
// to unblock and return an error.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, client := range s.clients {
		client.Close()
	}

	return s.listener.Close()
}

// Run runs the server, listening for connections until an error is returned.
func (s *Server) Run() error {
	errs := make(chan error)
	conns := make(chan net.Conn)

	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				errs <- err
				return
			}
			conns <- conn
		}
	}()

	for {
		select {
		case conn := <-conns:
			go s.runClient(conn, s.IRCDebug, s.DiscordDebug)
		case err := <-errs:
			return err
		}
	}
}

// runClient runs a client.Client on the given connection, with the debug
// settings given in its arguments.
//
// The debug arguments are given explicitly rather than taken from the shared
// Server state, because they are not guarded by the mutex.
func (s *Server) runClient(conn net.Conn, ircDebug, discordDebug bool) {
	cl := client.New(conn, s.session, ircDebug, discordDebug)
	defer cl.Close()

	if err := cl.Run(); err != nil {
		log.Println(err)
	}
}

// session returns the session.Session for the given token.
// It may be called concurrently from multiple goroutines.
//
// If debug is true and the session does not already exist, the returned
// session will log communication to the standard Logger.
func (s *Server) session(token string, debug bool) (*session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.ids[token]; ok {
		if sess, ok := s.sessions[id]; ok {
			sess.Ref()
			return sess, nil
		}
	}

	sess, err := session.New(token, debug, s.removeSession)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session")
	}

	me, err := sess.Me()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user from Discord")
	}

	s.ids[token] = me.ID

	for otherID, other := range s.sessions {
		if me.ID == otherID {
			sess.Close()
			return other, nil
		}
	}

	s.sessions[me.ID] = sess

	return sess, nil
}

// removeSession removes sess from the sessions map.
// It panics if sess was not present in the map.
// This function is meant to be passed in as the removeFunc in session.New.
func (s *Server) removeSession(sess *session.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, other := range s.sessions {
		if sess == other {
			delete(s.sessions, id)
			return
		}
	}

	panic("attempted to remove non-existent session")
}
