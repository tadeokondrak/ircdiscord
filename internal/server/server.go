package server

import (
	"fmt"
	"log"
	"net"

	"sync"

	"github.com/tadeokondrak/ircdiscord/internal/client"
	"github.com/tadeokondrak/ircdiscord/internal/session"
)

// Server is state shared across all connections.
type Server struct {
	listener     net.Listener
	debug        bool
	ircDebug     bool
	discordDebug bool

	mu       sync.Mutex                  // guards next 3 fields
	ids      map[string]int64            // tokens to IDs
	sessions map[int64]*session.Session  // IDs to sessions
	clients  map[*client.Client]struct{} // active clients
}

// New creates a new Server, taking ownership of listener.
func New(listener net.Listener, debug, ircDebug, discordDebug bool) *Server {
	if debug {
		log.Printf("creating server %v", listener.Addr())
	}

	return &Server{
		listener:     listener,
		debug:        debug,
		ircDebug:     ircDebug,
		discordDebug: discordDebug,
		ids:          make(map[string]int64),
		sessions:     make(map[int64]*session.Session),
		clients:      make(map[*client.Client]struct{}),
	}
}

// Close closes the server's listener, which in turn causes the current Run
// invocation to unblock and return an error.
func (s *Server) Close() error {
	if s.debug {
		log.Printf("destroying server %v", s.listener.Addr())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for cl := range s.clients {
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

// runClient runs a client.Client on conn.
func (s *Server) runClient(conn net.Conn) {
	cl := client.New(conn, s.session, s.debug, s.ircDebug, s.discordDebug)
	s.addClient(cl)
	defer s.removeClient(cl)

	if err := cl.Run(); err != nil {
		log.Printf("client %v disconnected: %v", conn.RemoteAddr(), err)
	}
}

// addClient adds cl to the client set.
func (s *Server) addClient(cl *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[cl] = struct{}{}
}

// removeClient is the lock-acquiring version of removeClientLock.
func (s *Server) removeClient(cl *client.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removeClientLock(cl)
}

// removeClientLock attempts to find cl in the client set,
// and removes it. It is a no-op if cl is not in the set.
// The caller must hold s.mu.
func (s *Server) removeClientLock(cl *client.Client) error {
	if _, ok := s.clients[cl]; ok {
		delete(s.clients, cl)
		s.mu.Unlock()
		err := cl.Close()
		s.mu.Lock()
		return err
	}

	return nil
}

// session returns the session.Session for token.
// It may be called concurrently from multiple goroutines.
// If debug is true and a new session is created, the newly created session will
// have debug enabled.
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
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.ids[token] = sess.UserID()

	for otherID, other := range s.sessions {
		if sess.UserID() == otherID {
			if s.debug {
				log.Printf("different token was used to log " +
					"into existing session, " +
					"closing new session")
			}

			sess.Close()
			return other, nil
		}
	}

	s.sessions[sess.UserID()] = sess

	go s.runSession(sess)

	return sess, nil
}

// runSesion runs sess.
func (s *Server) runSession(sess *session.Session) {
	if err := sess.Run(); err != nil {
		log.Println(err)
	}
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
