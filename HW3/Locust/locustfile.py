import random
from locust import FastHttpUser, task, between

class AlbumUser(FastHttpUser):
    # Simulate a wait time between tasks
    wait_time = between(1, 2)

    @task(3) # Read-heavy workload
    # Read operation
    def get_albums(self):
        self.client.get("/albums", name="/albums (GET)")

    @task(1)
    # Write operation
    def post_album(self):
        # Generate a unique album ID for each POST request
        album_id = str(random.randint(100, 100000))
        payload = {
            "id": album_id,
            "title": f"Load Test Album {album_id}",
            "artist": "Locust",
            "price": 99.99
        }
        self.client.post("/albums", json=payload, name="/albums (POST)")