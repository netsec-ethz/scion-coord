angular.module('scionApp')
    .controller('adminCtrl', ['$rootScope', '$scope', 'adminService', '$location',
        function ($rootScope, $scope, adminService, $location) {
            $scope.redirectIfNotAdmin = function () {
                if (!$rootScope.user["IsAdmin"]) {
                    $location.path('/user');
                }
            };

            $scope.adminPageData = function () {
                adminService.adminPageData().then(
                    function (data) {
                        console.log(data);
                        $rootScope.user = data["User"];
                        $scope.account = data["Account"];
                        // option to allow the email template to be changed
                        // $scope.emailTemplate = data["EmailTemplate"];
                        $scope.organisation = $rootScope.user["Organisation"];
                        $scope.emailMessage = data["EmailMessage"];

                        $scope.redirectIfNotAdmin();
                        $scope.defaultInvitation = function () {
                            return {
                                Organisation: $scope.organisation,
                                error: false
                            };
                        };
                        $scope.resetInvitations = function () {
                            $scope.invitations = [$scope.defaultInvitation()];
                        };
                        $scope.resetInvitations();
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/user');
                        }
                    });
            };

            $scope.adminPageData();
            $scope.error = "";
            $scope.message = "";

            $scope.addChoice = function () {
                $scope.invitations.push($scope.defaultInvitation());
            };

            $scope.removeChoice = function () {
                $scope.invitations.pop();
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };

            $scope.sendInvitations = function (invitations) {
                // TODO(mlegner): Maybe check for duplicates before api call.
                adminService.sendInvitations(invitations).then(
                    function (data) {
                        if (data["emails"].length === 0) {
                            $scope.error = "";
                            $scope.message = "All email invitations sent successfully.";
                            $scope.resetInvitations();
                        } else {
                            let emails = data["emails"];
                            let messages = data["messages"];
                            err = "There was a problem sending emails to the following addresses: ";
                            for (let i = 0; i < invitations.length; i++) {
                                invitations[i].error = messages[i];
                            }
                            for (let i = 0; i < emails.length; i++) {
                                err += emails[i];
                                if (i < emails.length - 1) {
                                    err += ", ";
                                }
                            }
                            $scope.error = err;
                        }
                    },
                    function (response) {
                        $scope.error = "There was an error sending email invitations.";
                        $scope.message = "";
                        console.log(response);
                    });
            };
        }
    ]);
