package pipeline

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestEmptyPipelineThenReturnIsIdentity(t *testing.T) {
	p := New[int]().ThenReturn()
	out, err := p(context.Background(), 42)
	if err != nil || out != 42 {
		t.Fatalf("identity: got %d err=%v", out, err)
	}
}

func TestStagesRunInAppendOrder(t *testing.T) {
	var log []string
	mk := func(label string) Stage[int] {
		return func(ctx context.Context, v int, next Next[int]) (int, error) {
			log = append(log, "pre-"+label)
			v, err := next(ctx, v+1)
			log = append(log, "post-"+label)
			return v, err
		}
	}
	out, err := New[int]().Through(mk("A"), mk("B"), mk("C")).
		Then(func(ctx context.Context, v int) (int, error) {
			log = append(log, fmt.Sprintf("final=%d", v))
			return v * 10, nil
		})(context.Background(), 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != 30 {
		t.Fatalf("out: %d", out)
	}
	expected := []string{"pre-A", "pre-B", "pre-C", "final=3", "post-C", "post-B", "post-A"}
	for i, want := range expected {
		if i >= len(log) || log[i] != want {
			t.Fatalf("log[%d]: want %q got %v", i, want, log)
		}
	}
}

func TestShortCircuit(t *testing.T) {
	var ran int
	stopErr := errors.New("stop")
	pipe := New[int]().Through(
		func(ctx context.Context, v int, next Next[int]) (int, error) {
			return 0, stopErr
		},
		func(ctx context.Context, v int, next Next[int]) (int, error) {
			ran++
			return next(ctx, v)
		},
	).Then(func(ctx context.Context, v int) (int, error) {
		ran++
		return v, nil
	})
	_, err := pipe(context.Background(), 1)
	if !errors.Is(err, stopErr) {
		t.Fatalf("expected stopErr, got %v", err)
	}
	if ran != 0 {
		t.Fatalf("subsequent stages ran: %d", ran)
	}
}

func TestSkipNilStages(t *testing.T) {
	pipe := New[int]().Through(nil, nil).ThenReturn()
	out, err := pipe(context.Background(), 7)
	if err != nil || out != 7 {
		t.Fatalf("nil-stage pipe: %d err=%v", out, err)
	}
}

func TestNilFinalUsesPassthrough(t *testing.T) {
	pipe := New[int]().Through(
		FuncStage(func(ctx context.Context, v int) {}),
	).Then(nil)
	out, err := pipe(context.Background(), 9)
	if err != nil || out != 9 {
		t.Fatalf("nil final: %d %v", out, err)
	}
}

func TestContextCanceledStagesShouldRespect(t *testing.T) {
	pipe := New[int]().Through(func(ctx context.Context, v int, next Next[int]) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return next(ctx, v)
	}).Then(func(ctx context.Context, v int) (int, error) { return v, nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pipe(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestFuncStageHelper(t *testing.T) {
	var seen atomic.Int32
	pipe := New[int]().
		Through(FuncStage(func(ctx context.Context, v int) { seen.Add(int32(v)) })).
		Through(FuncStage(func(ctx context.Context, v int) { seen.Add(int32(v) * 10) })).
		ThenReturn()
	out, _ := pipe(context.Background(), 3)
	if out != 3 {
		t.Fatalf("ThenReturn should not transform: %d", out)
	}
	if got := seen.Load(); got != 33 {
		t.Fatalf("FuncStage tally: %d", got)
	}
}

func TestStagesCount(t *testing.T) {
	p := New[int]().Through(
		FuncStage(func(ctx context.Context, v int) {}),
		nil,
		FuncStage(func(ctx context.Context, v int) {}),
	)
	if p.Stages() != 2 {
		t.Fatalf("Stages count: %d", p.Stages())
	}
}

func TestConcurrentPipeExecutions(t *testing.T) {
	pipe := New[int]().Through(
		func(ctx context.Context, v int, next Next[int]) (int, error) { return next(ctx, v+1) },
		func(ctx context.Context, v int, next Next[int]) (int, error) { return next(ctx, v*2) },
	).Then(func(ctx context.Context, v int) (int, error) { return v, nil })

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, _ := pipe(context.Background(), i)
			want := (i + 1) * 2
			if out != want {
				t.Errorf("i=%d want=%d got=%d", i, want, out)
			}
		}()
	}
	wg.Wait()
}

func TestHTTPChain(t *testing.T) {
	var seen []string
	mk := func(label string) HTTPStage {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = append(seen, "pre-"+label)
				next.ServeHTTP(w, r)
				seen = append(seen, "post-"+label)
			})
		}
	}
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "handler")
		_, _ = w.Write([]byte("ok"))
	})
	h := Chain(final, mk("A"), nil, mk("B"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Body.String() != "ok" {
		t.Fatalf("body: %q", rec.Body.String())
	}
	got := strings.Join(seen, ",")
	want := "pre-A,pre-B,handler,post-B,post-A"
	if got != want {
		t.Fatalf("chain order: %s", got)
	}
}
