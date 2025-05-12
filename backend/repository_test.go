package main

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

var r *Repositories

func TestMain(m *testing.M) {
	// 1. Validate required environment variables
	if os.Getenv("MONGO_URI") == "" {
		log.Fatal("MONGO_URI must be set (source env.sh)")
	}
	if os.Getenv("REDIS_URL") == "" {
		log.Fatal("REDIS_URL must be set (source env.sh)")
	}
	if os.Getenv("DB_NAME") == "" {
		log.Fatal("DB_NAME must be set (source env.sh)")
	}

	// 2. Initialize OTEL, Mongo, Redis, and your Repositories
	ctx := context.Background()

	otelShutDown, err := initOtelSDK(ctx)
	if err != nil {
		log.Fatalf("initOtelSDK error: %v", err)
	}

	mongoShutDown, err := initDB(ctx)
	if err != nil {
		log.Fatalf("initDB error: %v", err)
	}

	redisShutDown, err := initRedis(ctx)
	if err != nil {
		log.Fatalf("initRedis error: %v", err)
	}

	if err = InitCachedMongoRepositories(ctx, RedisClient, MongoClient, 15*time.Minute); err != nil {
		log.Fatalf("cache init error: %v", err)
	}

	if r, err = initMongoRepositories(MongoClient); err != nil {
		log.Fatalf("initMongoRepositories error: %v", err)
	}

	// 3. Run tests
	code := m.Run()

	// 4. Teardown: shut down clients, flush logs, etc.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Waiting for OTEL logs to flush (8 seconds)...")
		time.Sleep(8 * time.Second)
		log.Printf("Done waiting for OTEL logs")
	}()

	log.Printf("Waiting for background tasks to complete...")
	wg.Wait()
	log.Printf("All background tasks completed")

	log.Printf("Cleaning up\n")
	err = errors.Join(otelShutDown(ctx), err)
	err = errors.Join(mongoShutDown(ctx), err)
	err = errors.Join(redisShutDown(ctx), err)

	if err != nil {
		log.Printf("Error in cleaning up resources -> %v\n", err)
	} else {
		log.Printf("Cleaned up resources successfully\n")
	}

	log.Printf("Exit code -> %d\n", code)
	os.Exit(code)
}

func TestUserCreate(t *testing.T) {
	in := generateUser(3, 5)
	id, err := r.User.CreateUser(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value

	out, err := r.User.FindUserByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkUserStruct(in, out); err != nil {
		t.Fatal(err)
	}
}

func TestVendorCreate(t *testing.T) {
	in := generateVendor(3, 5)
	id, err := r.Vendor.CreateVendor(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value

	out, err := r.Vendor.FindVendorByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkVendorStruct(in, out); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCreate(t *testing.T) {
	in := generateAdmin(3)
	id, err := r.Admin.CreateAdmin(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value

	out, err := r.Admin.FindAdminByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAdminStruct(in, out); err != nil {
		t.Fatal(err)
	}
}

func TestUserDelete(t *testing.T) {
	in := generateUser(3, 5)
	id, err := r.User.CreateUser(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value

	err = r.User.DeleteUser(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.User.FindUserByID(context.Background(), ID{in.ID}); err == nil {
		t.Fatal(err)
	}
}

func TestVendorDelete(t *testing.T) {
	in := generateVendor(3, 5)
	id, err := r.Vendor.CreateVendor(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value

	err = r.Vendor.DeleteVendor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Vendor.FindVendorByID(context.Background(), ID{in.ID}); err == nil {
		t.Fatal(err)
	}
}

func TestAdminDelete(t *testing.T) {
	in := generateAdmin(3)
	id, err := r.Admin.CreateAdmin(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.ID = id.value
	err = r.Admin.Delete(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Admin.FindAdminByID(context.Background(), ID{in.ID}); err == nil {
		t.Fatal(err)
	}
}
