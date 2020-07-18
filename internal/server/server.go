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

// Server is state shared across all connections.
type Server struct {
	listener     net.Listener
	debug        bool
	ircDebug     bool
	discordDebug bool

	mu       sync.Mutex                             // guards next 3 fields
	ids      map[string]discord.Snowflake           // tokens to IDs
	sessions map[discord.Snowflake]*session.Session // IDs to sessions
	clients  []*client.Client                       // active clients
}

// New creates a new Server, taking ownership of the listener.
func New(listener net.Listener, debug, ircDebug, discordDebug bool) *Server {
	if debug {
		log.Printf("creating server %v", listener.Addr())
	}

	return &Server{
		debug:        debug,
		ircDebug:     ircDebug,
		discordDebug: discordDebug,
		listener:     listener,
		ids:          make(map[string]discord.Snowflake),
		sessions:     make(map[discord.Snowflake]*session.Session),
	}
}

// Close closes the server's listener, which causes the current Run invocation
// to unblock and return an error.
func (s *Server) Close() error {
	if s.debug {
		log.Printf("destroying server %v", s.listener.Addr())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cl := range s.clients {
		s.removeClientLock(cl)
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
			go s.runClient(conn)
		case err := <-errs:
			return err
		}
	}
}

// runClient runs a client.Client on the given connection, with the debug
// settings given in its arguments.
func (s *Server) runClient(conn net.Conn) {
	cl := client.New(conn, s.session, s.debug, s.ircDebug, s.discordDebug)
	s.mu.Lock()
	s.clients = append(s.clients, cl)
	s.mu.Unlock()
	defer s.removeClient(cl)

	if err := cl.Run(); err != nil {
		log.Printf("client %v disconnected with error: %v",
			conn.RemoteAddr(), err)
	}
}

// removeClient is the lock-acquiring version of removeClientLock.
func (s *Server) removeClient(cl *client.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeClientLock(cl)
}

// removeClientLock attempts to find a client in the list of clients,
// and removes it. It is a no-op if the client is not in the list.
// The caller must hold s.mu.
func (s *Server) removeClientLock(cl *client.Client) error {
	for i, other := range s.clients {
		if cl == other {
			// remove
			s.clients[i] = s.clients[len(s.clients)-1]
			s.clients[len(s.clients)-1] = nil
			s.clients = s.clients[:len(s.clients)-1]

			s.mu.Unlock()
			err := cl.Close()
			s.mu.Lock()
			return err
		}
	}

	return nil
}

// session returns the session.Session for the given token.
// It may be called concurrently from multiple goroutines.
func (s *Server) session(token string, debug bool) (*session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.debug {
		log.Printf("requested session for token %s", token)
	}

	if id, ok := s.ids[token]; ok {
		if s.debug {
			log.Printf("found id %v for token %s", id, token)
		}

		if sess, ok := s.sessions[id]; ok {
			if s.debug {
				log.Printf("found session %p for token %s",
					sess, token)
			}

			sess.Ref()
			return sess, nil
		}
	}

	if s.debug {
		log.Printf("no session for token %s found, creating one", token)
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
			if s.debug {
				log.Printf("different token was used to log " +
					"into existing session, " +
					"closing new session")
			}

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

	if s.debug {
		log.Printf("removing session %p from server %v",
			sess, s.listener.Addr())
	}

	for id, other := range s.sessions {
		if sess == other {
			delete(s.sessions, id)
			return
		}
	}

	panic("attempted to remove non-existent session")
}
