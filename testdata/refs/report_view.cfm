<cfset myCtrl = new controller()>
<cfset report = myCtrl.RunReport(id=URL.id)>
<cfoutput>#report.name#</cfoutput>
