'use client';

import { useState } from 'react';
import { Upload, X } from 'lucide-react';

interface Props {
  studioId: string;
  onImageUpload: (mediaUrl: string) => void;
  onError?: (error: string) => void;
}

export function SocialPostImageUploader({ studioId, onImageUpload, onError }: Props) {
  const [uploading, setUploading] = useState(false);
  const [preview, setPreview] = useState<string>();
  const [error, setError] = useState<string>();

  async function handleUpload(file: File) {
    setError(undefined);

    const validTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
    if (!validTypes.includes(file.type)) {
      const msg = 'Only JPEG, PNG, WebP, and GIF images are allowed';
      setError(msg);
      onError?.(msg);
      return;
    }

    if (file.size > 10 * 1024 * 1024) {
      const msg = 'File size must be less than 10MB';
      setError(msg);
      onError?.(msg);
      return;
    }

    setUploading(true);
    const formData = new FormData();
    formData.append('file', file);

    try {
      const res = await fetch(`/api/v1/studios/${studioId}/social-posts/upload-image`, {
        method: 'POST',
        body: formData,
      });

      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(`Upload failed: ${res.status} ${errorText}`);
      }

      const contentType = res.headers.get('content-type');
      if (!contentType?.includes('application/json')) {
        throw new Error(`Invalid response type: ${contentType}`);
      }

      const data = await res.json() as { mediaUrl: string };
      if (!data.mediaUrl) {
        throw new Error('No mediaUrl in response');
      }
      onImageUpload(data.mediaUrl);
      setPreview(data.mediaUrl);
    } catch (err) {
      const msg = (err as Error).message || 'Failed to upload image';
      setError(msg);
      onError?.(msg);
    } finally {
      setUploading(false);
    }
  }

  function handleRemove() {
    setPreview(undefined);
    onImageUpload('');
  }

  return (
    <div className="space-y-3">
      <label className="block text-sm font-medium text-slate-900 dark:text-white">
        Post Image <span className="text-slate-500">(optional)</span>
      </label>

      {preview ? (
        <div className="relative w-full h-48 rounded-lg overflow-hidden border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800">
          <img
            src={preview}
            alt="Preview"
            className="w-full h-full object-cover"
            onError={(e) => {
              console.error('Preview image failed to load:', preview);
              (e.target as HTMLElement).style.display = 'none';
            }}
            onLoad={() => {
              console.log('Preview image loaded:', preview);
            }}
          />
          <button
            type="button"
            onClick={handleRemove}
            disabled={uploading}
            className="absolute top-2 right-2 p-1.5 bg-red-500 hover:bg-red-600 text-white rounded-full shadow-lg transition-colors disabled:opacity-50"
          >
            <X className="h-4 w-4" />
          </button>
          {uploading && (
            <div className="absolute inset-0 bg-black/30 flex items-center justify-center">
              <div className="animate-spin h-6 w-6 border-2 border-white border-t-transparent rounded-full" />
            </div>
          )}
        </div>
      ) : (
        <label className="flex flex-col items-center justify-center w-full h-48 border-2 border-dashed border-slate-300 dark:border-slate-600 rounded-lg cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-900/50 transition-colors">
          <div className="flex flex-col items-center justify-center pt-5 pb-6 px-4">
            <Upload className="h-8 w-8 text-slate-400 mb-2" />
            <p className="text-sm font-medium text-slate-900 dark:text-white">Click to upload or drag and drop</p>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">PNG, JPG, WebP, GIF up to 10MB</p>
          </div>
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            onChange={(e) => e.target.files?.[0] && handleUpload(e.target.files[0])}
            disabled={uploading}
            className="hidden"
            aria-label="Upload image"
          />
        </label>
      )}

      {uploading && !preview && (
        <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
          <div className="animate-spin h-4 w-4 border-2 border-slate-300 dark:border-slate-600 border-t-slate-900 dark:border-t-white rounded-full" />
          <span>Uploading...</span>
        </div>
      )}

      {error && (
        <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
      )}
    </div>
  );
}
