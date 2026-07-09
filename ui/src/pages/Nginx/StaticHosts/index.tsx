import { HasPermission } from "src/components";
import { STATIC_HOSTS, VIEW } from "src/modules/Permissions";
import TableWrapper from "./TableWrapper";

const StaticHosts = () => {
	return (
		<HasPermission section={STATIC_HOSTS} permission={VIEW} pageLoading loadingNoLogo>
			<TableWrapper />
		</HasPermission>
	);
};

export default StaticHosts;
