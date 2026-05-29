"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { FaTwitter } from "react-icons/fa";
import { AlertCircle } from "lucide-react";

import { useApi } from "@/hooks/useApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

interface ConnectXProps {
  studioId: string;
  onSuccess: () => void;
}

export function ConnectX({ studioId, onSuccess }: ConnectXProps) {
  const [consumerKey, setConsumerKey] = useState("");
  const [consumerSecret, setConsumerSecret] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [accessTokenSecret, setAccessTokenSecret] = useState("");
  const [xHandle, setXHandle] = useState("");
  const [error, setError] = useState<string | null>(null);
  const { fetchWithAuth, isLoading } = useApi();
  const router = useRouter();

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      const response = await fetchWithAuth(
        `/api/v1/studios/${studioId}/channels/x`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            consumer_key: consumerKey,
            consumer_secret: consumerSecret,
            access_token: accessToken,
            access_token_secret: accessTokenSecret,
            x_handle: xHandle,
          }),
        }
      );

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Failed to connect X channel");
      }

      onSuccess();
      router.refresh();
    } catch (err: any) {
      setError(err.message || "An unexpected error occurred.");
    }
  };

  return (
    <Card className="max-w-2xl mt-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-xl">
          <FaTwitter className="w-6 h-6 text-sky-500" />
          Connect X (Twitter) Direct Messages
        </CardTitle>
        <CardDescription>
          Enter your X API OAuth 1.0a App Keys to connect your account and
          receive Direct Messages. Note: X requires the Basic API Tier to send
          and receive DMs.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {error && (
          <Alert variant="destructive" className="mb-6">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <form onSubmit={handleConnect} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Consumer Key (API Key)</label>
            <Input
              value={consumerKey}
              onChange={(e) => setConsumerKey(e.target.value)}
              placeholder="e.g. mfnwJ9..."
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Consumer Secret (API Secret)</label>
            <Input
              type="password"
              value={consumerSecret}
              onChange={(e) => setConsumerSecret(e.target.value)}
              placeholder="Enter your Consumer Secret"
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Access Token</label>
            <Input
              value={accessToken}
              onChange={(e) => setAccessToken(e.target.value)}
              placeholder="e.g. 206025709..."
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Access Token Secret</label>
            <Input
              type="password"
              value={accessTokenSecret}
              onChange={(e) => setAccessTokenSecret(e.target.value)}
              placeholder="Enter your Access Token Secret"
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">X Handle (without @)</label>
            <Input
              value={xHandle}
              onChange={(e) => setXHandle(e.target.value)}
              placeholder="e.g. PuneethGMt2"
              required
            />
          </div>

          <Button type="submit" disabled={isLoading} className="w-full">
            {isLoading ? "Connecting..." : "Connect X Account"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
