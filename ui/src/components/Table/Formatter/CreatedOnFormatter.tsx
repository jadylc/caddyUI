import { formatDateTime } from "src/locale";

interface Props {
	value: string | number;
}

export function CreatedOnFormatter({ value }: Props) {
	if (!value) {
		return <span className="text-muted">-</span>;
	}
	return <span className="text-secondary text-nowrap">{formatDateTime(value)}</span>;
}
