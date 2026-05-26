// EMVCo-compliant PromptPay QR payload builder.
//
// Format reference: Bank of Thailand "Standard for QR Code Payment in Thailand"
// (ISO 18245 / EMVCo MPM 04.00). Each field is a 2-digit tag, 2-digit decimal
// length, then the value. A CRC-16/CCITT-FALSE checksum (poly 0x1021, init
// 0xFFFF) covers everything including the trailing tag 63 "6304" prefix.

const AID_PROMPTPAY = "A000000677010111";

function tlv(tag: string, value: string): string {
  return tag + value.length.toString().padStart(2, "0") + value;
}

// crc16ccittFalse computes CRC-16/CCITT-FALSE (poly 0x1021, init 0xFFFF, no
// reflection, no xor-out) over the input bytes — the variant the PromptPay
// spec calls for.
function crc16(input: string): string {
  let crc = 0xffff;
  for (let i = 0; i < input.length; i++) {
    crc ^= input.charCodeAt(i) << 8;
    for (let j = 0; j < 8; j++) {
      crc = crc & 0x8000 ? (crc << 1) ^ 0x1021 : crc << 1;
      crc &= 0xffff;
    }
  }
  return crc.toString(16).toUpperCase().padStart(4, "0");
}

// formatTarget normalizes a raw PromptPay ID (phone / NID / tax-ID / e-wallet)
// into the 13-or-15-digit form the spec requires.
//   - 10-digit Thai phone (must start with 0)  → 13 chars: "0066" + last 9 digits
//   - 13-digit national ID                     → 13 chars unchanged
//   - 15-digit tax ID or e-wallet              → 15 chars unchanged
// Anything else → null (caller should fall back to the placeholder payload).
function formatTarget(rawID: string): string | null {
  const digits = rawID.replace(/\D/g, "");
  if (digits.length === 10 && digits.startsWith("0")) {
    return "0066" + digits.slice(1);
  }
  if (digits.length === 13 || digits.length === 15) {
    return digits;
  }
  return null;
}

// formatAmount converts integer satang to the "1500.00" decimal-string form
// EMVCo's tag-54 expects. Caller must guarantee non-negative input.
function formatAmount(cents: number): string {
  const whole = Math.floor(cents / 100);
  const frac = cents % 100;
  return `${whole}.${frac.toString().padStart(2, "0")}`;
}

export type PromptPayPayloadInput = {
  promptpayID: string;
  amountCents: number;
};

// buildPromptPayPayload returns the EMVCo string to feed into a QR encoder, or
// null when the merchant ID can't be parsed (caller renders no QR and shows the
// guest the booking reference for manual reconciliation).
export function buildPromptPayPayload({
  promptpayID,
  amountCents,
}: PromptPayPayloadInput): string | null {
  const target = formatTarget(promptpayID);
  if (!target) return null;

  const merchantInfo = tlv("00", AID_PROMPTPAY) + tlv(target.length === 15 ? "03" : "01", target);

  const fields = [
    tlv("00", "01"),
    tlv("01", "12"),
    tlv("29", merchantInfo),
    tlv("53", "764"),
    tlv("54", formatAmount(amountCents)),
    tlv("58", "TH"),
  ].join("");

  // CRC is computed over everything plus the "6304" prefix that introduces the
  // CRC field itself; the spec is explicit on this.
  const withCrcPlaceholder = fields + "6304";
  return withCrcPlaceholder + crc16(withCrcPlaceholder);
}
