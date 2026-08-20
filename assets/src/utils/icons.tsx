import type { FunctionalComponent } from "preact";
import {
  PlayIcon,
  PauseIcon,
  ArrowPathIcon,
  Cog6ToothIcon,
  ClockIcon,
  GlobeAltIcon,
  StarIcon,
  TrashIcon,
  PlusIcon,
  CheckIcon,
  XMarkIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  ArrowLeftIcon,
  ChartBarIcon,
  ArchiveBoxIcon,
  PencilSquareIcon,
  ClipboardDocumentCheckIcon,
  PhotoIcon,
  ForwardIcon,
} from "@heroicons/react/24/outline";

export const PlayIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <PlayIcon className={className} />
);

export const PauseIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <PauseIcon className={className} />
);

export const ResetIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ArrowPathIcon className={className} />
);

export const SettingsIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <Cog6ToothIcon className={className} />
);

export const ClockIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ClockIcon className={className} />
);

export const TimerIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ClockIcon className={className} />
);

export const GlobeIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <GlobeAltIcon className={className} />
);

export const StarIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <StarIcon className={className} />
);

export const TrashIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <TrashIcon className={className} />
);

export const PlusIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <PlusIcon className={className} />
);

export const CheckIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <CheckIcon className={className} />
);

export const CloseIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <XMarkIcon className={className} />
);

export const ChevronDownIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ChevronDownIcon className={className} />
);

export const ChevronUpIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ChevronUpIcon className={className} />
);

export const ArrowLeftIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ArrowLeftIcon className={className} />
);

export const ChartIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ChartBarIcon className={className} />
);

export const BackupIcon: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ArchiveBoxIcon className={className} />
);

export const PencilIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <PencilSquareIcon className={className} />
);

export const HabitsIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ClipboardDocumentCheckIcon className={className} />
);

export const PhotoIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <PhotoIcon className={className} />
);

export const ForwardIconComponent: FunctionalComponent<{ className?: string }> = ({ className = "w-5 h-5" }) => (
  <ForwardIcon className={className} />
);
