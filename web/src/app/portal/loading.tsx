import { Skeleton } from '@/components/ui/skeleton';

export default function PortalLoading() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <Skeleton className="w-[300px] h-6" />
    </div>
  );
}
