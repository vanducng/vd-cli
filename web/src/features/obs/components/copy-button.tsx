import { useState } from "react";
import { Check, Copy } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const FLASH_MS = 1500;

interface CopyButtonProps {
  value: string;
  label?: string;
  className?: string;
}

/** Copies `value` to the clipboard and flashes a checkmark for FLASH_MS, used
 * for skill paths and the generated investigation prompt on /obs/health. */
export function CopyButton({ value, label, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), FLASH_MS);
    } catch {
      /* clipboard unavailable (unfocused doc, insecure context) — leave the label unchanged */
    }
  }

  return (
    <Button type="button" variant="outline" size="sm" onClick={handleCopy} className={cn("h-7 gap-1.5 px-2 text-xs", className)}>
      {copied ? <Check className="h-3 w-3 text-ok" /> : <Copy className="h-3 w-3" />}
      {copied ? "Copied" : (label ?? "Copy")}
    </Button>
  );
}
