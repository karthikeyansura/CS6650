"""
Locust load test for ChaosArena v1 Album Store.

Approximates S11-S15 scenarios:
  S11: concurrent album creates (PUT /albums)
  S12: concurrent photo uploads (POST -> poll until completed)
  S13: mixed read/write metadata (GET/PUT /albums)
  S14: mixed metadata + uploads simultaneously
  S15: large payload uploads

Usage:
  locust -f locustfile.py --host=http://<ALB_DNS>
"""

import uuid
import io
import random
import time

from locust import HttpUser, task, between, events


# pre-created album IDs for photo upload tasks
SHARED_ALBUMS = [str(uuid.uuid4()) for _ in range(20)]


class AlbumStoreUser(HttpUser):
    wait_time = between(0.1, 0.5)

    def on_start(self):
        """Seed a few albums so photo upload tasks have targets."""
        for album_id in SHARED_ALBUMS[:5]:
            self.client.put(
                f"/albums/{album_id}",
                json={
                    "album_id": album_id,
                    "title": f"Seed Album {album_id[:8]}",
                    "description": "Seeded for load testing",
                    "owner": "locust@northeastern.edu",
                },
                name="PUT /albums/:id [seed]",
            )

    # ------------------------------------------------------------------ S11
    @task(5)
    def create_album(self):
        album_id = str(uuid.uuid4())
        self.client.put(
            f"/albums/{album_id}",
            json={
                "album_id": album_id,
                "title": f"Album {random.randint(1, 100000)}",
                "description": "Load test album",
                "owner": "locust@northeastern.edu",
            },
            name="PUT /albums/:id",
        )

    # ------------------------------------------------------------------ S13
    @task(3)
    def get_album(self):
        album_id = random.choice(SHARED_ALBUMS)
        self.client.get(f"/albums/{album_id}", name="GET /albums/:id")

    @task(2)
    def list_albums(self):
        self.client.get("/albums", name="GET /albums")

    # ------------------------------------------------------------------ S12 / S14
    @task(3)
    def upload_and_poll(self):
        album_id = random.choice(SHARED_ALBUMS)
        # 1KB dummy photo
        photo_data = io.BytesIO(b"x" * 1024)
        with self.client.post(
            f"/albums/{album_id}/photos",
            files={"photo": ("test.jpg", photo_data, "image/jpeg")},
            name="POST /albums/:id/photos",
            catch_response=True,
        ) as resp:
            if resp.status_code != 202:
                resp.failure(f"Expected 202, got {resp.status_code}")
                return
            body = resp.json()
            photo_id = body.get("photo_id")

        if not photo_id:
            return

        # poll until completed or timeout (30s)
        start = time.time()
        while time.time() - start < 30:
            with self.client.get(
                f"/albums/{album_id}/photos/{photo_id}",
                name="GET /albums/:id/photos/:id [poll]",
                catch_response=True,
            ) as poll_resp:
                if poll_resp.status_code == 200:
                    data = poll_resp.json()
                    if data.get("status") == "completed":
                        poll_resp.success()
                        return
                    elif data.get("status") == "failed":
                        poll_resp.failure("Photo processing failed")
                        return
            time.sleep(0.5)

    # ------------------------------------------------------------------ S15
    @task(1)
    def upload_large(self):
        album_id = random.choice(SHARED_ALBUMS)
        # 5MB payload to approximate large upload behavior
        photo_data = io.BytesIO(b"L" * (5 * 1024 * 1024))
        with self.client.post(
            f"/albums/{album_id}/photos",
            files={"photo": ("large.jpg", photo_data, "image/jpeg")},
            name="POST /albums/:id/photos [large]",
            catch_response=True,
        ) as resp:
            if resp.status_code != 202:
                resp.failure(f"Expected 202, got {resp.status_code}")

    # ------------------------------------------------------------------ health
    @task(1)
    def health(self):
        self.client.get("/health", name="GET /health")
