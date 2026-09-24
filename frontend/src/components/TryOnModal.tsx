"use client";

import { useEffect, useRef, useState } from "react";
import type { FaceLandmarker } from "@mediapipe/tasks-vision";
import { API_URL } from "@/lib/api";
import type { Product } from "@/lib/products";

type TryOnModalProps = {
  product: Product;
  onClose: () => void;
};

type Placement = {
  x: number;
  y: number;
  scale: number;
  rotation: number;
};

const defaultPlacement: Placement = { x: 50, y: 43, scale: 1, rotation: 0 };
let faceLandmarkerPromise: Promise<FaceLandmarker> | null = null;

export function TryOnModal({ product, onClose }: TryOnModalProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const imageRef = useRef<HTMLImageElement | null>(null);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [photo, setPhoto] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [placement, setPlacement] = useState<Placement>(defaultPlacement);
  const mockupSrc = toImageSrc(product.tryOnImageUrl);

  useEffect(() => {
    return () => stream?.getTracks().forEach((track) => track.stop());
  }, [stream]);

  async function openCamera() {
    setError("");
    setStatus("");
    try {
      const media = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "user" }, audio: false });
      setStream(media);
      if (videoRef.current) videoRef.current.srcObject = media;
    } catch {
      setError("Camera permission was denied or unavailable. You can upload a photo instead.");
    }
  }

  function takePhoto() {
    if (!videoRef.current || !canvasRef.current || videoRef.current.videoWidth === 0) return;
    const canvas = canvasRef.current;
    const video = videoRef.current;
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    canvas.getContext("2d")?.drawImage(video, 0, 0);
    setPhoto(canvas.toDataURL("image/png"));
  }

  function uploadPhoto(file?: File) {
    if (!file) return;
    setError("");
    const reader = new FileReader();
    reader.onload = () => setPhoto(String(reader.result));
    reader.readAsDataURL(file);
  }

  async function detectFace() {
    if (!imageRef.current) return;
    setStatus("Detecting face...");
    try {
      const detector = await getFaceLandmarker();
      const result = detector.detect(imageRef.current);
      const landmarks = result.faceLandmarks[0];
      if (!landmarks) {
        setStatus("Face detection could not find a face. You can adjust the mockup manually.");
        setPlacement(defaultPlacement);
        return;
      }

      const leftEye = landmarks[33];
      const rightEye = landmarks[263];
      const noseBridge = landmarks[168] ?? landmarks[6];
      const centerX = ((leftEye.x + rightEye.x) / 2) * 100;
      const centerY = ((leftEye.y + rightEye.y + noseBridge.y) / 3) * 100;
      const eyeDx = rightEye.x - leftEye.x;
      const eyeDy = rightEye.y - leftEye.y;
      const eyeDistance = Math.hypot(eyeDx, eyeDy);
      const rotation = (Math.atan2(eyeDy, eyeDx) * 180) / Math.PI;

      setPlacement({
        x: clamp(centerX, 20, 80),
        y: clamp(centerY, 20, 70),
        scale: clamp(eyeDistance / 0.28, 0.65, 1.6),
        rotation: clamp(rotation, -25, 25),
      });
      setStatus("Face detected. You can fine tune the fit manually.");
    } catch {
      setStatus("Face detection is unavailable in this browser session. You can adjust the mockup manually.");
      setPlacement(defaultPlacement);
    }
  }

  function updatePlacement(key: keyof Placement, value: number) {
    setPlacement((current) => ({ ...current, [key]: value }));
  }

  function reset() {
    setPlacement(defaultPlacement);
    if (photo) void detectFace();
  }

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-black/70 p-4">
      <div className="mx-auto max-w-5xl rounded-xl bg-white p-5 shadow-2xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-2xl font-black">Try on {product.name}</h2>
            <p className="mt-2 text-sm text-neutral-600">Your photo is processed on your device for preview only and is not uploaded or saved.</p>
          </div>
          <button className="rounded-md border px-3 py-2 font-black" onClick={onClose}>Close</button>
        </div>

        {error ? <p className="mt-4 rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
        {status ? <p className="mt-4 rounded-md bg-[#f0f7f4] p-3 text-sm font-bold text-[#173c34]">{status}</p> : null}

        <div className="mt-5 grid gap-5 lg:grid-cols-[1fr_280px]">
          <div className="relative overflow-hidden rounded-lg bg-neutral-100">
            {!photo ? (
              <video ref={videoRef} autoPlay playsInline muted className="aspect-[4/3] w-full object-cover" />
            ) : (
              <div className="relative">
                <img ref={imageRef} alt="Face preview" className="aspect-[4/3] w-full object-cover" src={photo} onLoad={detectFace} />
                <GlassesOverlay placement={placement} src={mockupSrc} />
              </div>
            )}
            <canvas ref={canvasRef} className="hidden" />
          </div>

          <div className="grid gap-3 self-start">
            <button className="rounded-md bg-neutral-950 px-4 py-3 text-sm font-black text-white" onClick={openCamera}>Open Camera</button>
            <button className="rounded-md border border-[#173c34] bg-[#f0f7f4] px-4 py-3 text-sm font-black text-[#173c34]" onClick={takePhoto}>Take Photo</button>
            <label className="rounded-md border border-neutral-300 bg-white px-4 py-3 text-center text-sm font-black">
              Upload Photo
              <input className="hidden" type="file" accept="image/*" onChange={(event) => uploadPhoto(event.target.files?.[0])} />
            </label>
            <Control label="Left / Right" value={placement.x} min={20} max={80} onChange={(value) => updatePlacement("x", value)} />
            <Control label="Up / Down" value={placement.y} min={20} max={70} onChange={(value) => updatePlacement("y", value)} />
            <Control label="Resize" value={placement.scale} min={0.55} max={1.8} step={0.05} onChange={(value) => updatePlacement("scale", value)} />
            <Control label="Rotate" value={placement.rotation} min={-25} max={25} onChange={(value) => updatePlacement("rotation", value)} />
            <button className="rounded-md border px-4 py-3 text-sm font-black" onClick={reset}>Reset</button>
            {!mockupSrc ? <p className="rounded-md bg-amber-50 p-3 text-xs font-bold leading-5 text-[#7b531c]">This product has no try-on mockup image yet, so a placeholder frame is shown.</p> : null}
          </div>
        </div>
      </div>
    </div>
  );
}

async function getFaceLandmarker() {
  if (!faceLandmarkerPromise) {
    faceLandmarkerPromise = import("@mediapipe/tasks-vision").then(async ({ FaceLandmarker, FilesetResolver }) => {
      const fileset = await FilesetResolver.forVisionTasks("https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@0.10.35/wasm");
      const options = {
        baseOptions: {
          modelAssetPath: "https://storage.googleapis.com/mediapipe-models/face_landmarker/face_landmarker/float16/latest/face_landmarker.task",
          delegate: "GPU" as const,
        },
        runningMode: "IMAGE" as const,
        numFaces: 1,
      };
      try {
        return await FaceLandmarker.createFromOptions(fileset, options);
      } catch {
        return FaceLandmarker.createFromOptions(fileset, {
          ...options,
          baseOptions: { ...options.baseOptions, delegate: "CPU" },
        });
      }
    });
  }
  return faceLandmarkerPromise;
}

function Control({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="grid gap-2 text-sm font-bold">
      {label}
      <input type="range" min={min} max={max} step={step} value={value} onChange={(event) => onChange(Number(event.target.value))} />
    </label>
  );
}

function GlassesOverlay({ placement, src }: { placement: Placement; src: string }) {
  return (
    <div
      className="absolute"
      style={{
        left: `${placement.x}%`,
        top: `${placement.y}%`,
        width: `${42 * placement.scale}%`,
        transform: `translate(-50%, -50%) rotate(${placement.rotation}deg)`,
      }}
    >
      {src ? <img alt="Try-on eyewear mockup" className="w-full select-none" draggable={false} src={src} /> : <DefaultGlasses />}
    </div>
  );
}

function DefaultGlasses() {
  return (
    <div className="relative aspect-[3.5/1] w-full">
      <div className="absolute left-0 top-1/2 h-[80%] w-[42%] -translate-y-1/2 rounded-full border-[9px] border-neutral-950/85 bg-white/10" />
      <div className="absolute right-0 top-1/2 h-[80%] w-[42%] -translate-y-1/2 rounded-full border-[9px] border-neutral-950/85 bg-white/10" />
      <div className="absolute left-1/2 top-1/2 h-[18%] w-[18%] -translate-x-1/2 -translate-y-1/2 border-t-[7px] border-neutral-950/85" />
    </div>
  );
}

function toImageSrc(url?: string) {
  if (!url) return "";
  if (url.startsWith("http")) return url;
  return `${API_URL}${url}`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}
